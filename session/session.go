package session

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/task"
)

// The kinds of a session.
const (
	Conn Kind = iota + 1 // Conn: a connection session
	User                 // User: a user session
)

// Kind is the kind of a session.
type Kind int

// String returns the name of the session kind.
func (k Kind) String() string {
	switch k {
	case Conn:
		return "conn"
	case User:
		return "user"
	}

	return ""
}

// Session manages the connection and user sessions of a node along with their channel
// subscriptions.
type Session struct {
	rw       sync.RWMutex                         // Read-write lock protecting the session maps
	conns    map[int64]network.Conn               // Connection sessions, keyed by connection ID
	users    map[int64]network.Conn               // User sessions, keyed by user ID
	channels map[string]map[network.Conn]struct{} // Channel subscriptions, keyed by channel name
}

// NewSession creates a session instance.
func NewSession() *Session {
	return &Session{
		conns:    make(map[int64]network.Conn),
		users:    make(map[int64]network.Conn),
		channels: make(map[string]map[network.Conn]struct{}),
	}
}

// AddConn adds conn to the connection sessions.
func (s *Session) AddConn(conn network.Conn) {
	s.rw.Lock()
	s.conns[conn.ID()] = conn
	s.rw.Unlock()
}

// RemConn removes conn from the connection sessions. It reports whether conn was registered and
// has been removed.
//
// The existence check and the removal happen under the same write lock, so the caller does not
// need to call [Session.Has] before RemConn and pay for a second lookup. When the same conn is
// removed concurrently, only the first call reports true, which keeps repeated removals from
// unbalancing the counters.
func (s *Session) RemConn(conn network.Conn) bool {
	s.rw.Lock()
	defer s.rw.Unlock()

	cid, uid := conn.ID(), conn.UID()

	if _, ok := s.conns[cid]; !ok {
		return false
	}

	delete(s.conns, cid)

	if uid != 0 {
		if c, ok := s.users[uid]; ok && c == conn {
			delete(s.users, uid)
		}
	}

	s.doClearConnAttrs(conn)

	return true
}

// Has reports whether the session identified by target exists. The kind selects whether target is
// a connection ID or a user ID. It reports [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *Session) Has(kind Kind, target int64) (ok bool, err error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	switch kind {
	case Conn:
		_, ok = s.conns[target]
	case User:
		_, ok = s.users[target]
	default:
		err = errors.ErrInvalidSessionKind
	}

	return
}

// Bind binds the user ID uid to the connection identified by cid. When uid is already bound to
// another connection, that connection is closed and replaced.
func (s *Session) Bind(cid, uid int64) error {
	s.rw.Lock()
	old, err := s.bind(cid, uid)
	s.rw.Unlock()

	if err != nil {
		return err
	}

	if old != nil {
		if err = old.Close(true); err != nil {
			log.Warnf("close conn failed: cid = %d, uid = %d, err = %v", cid, uid, err)
		}
	}

	return nil
}

// bind binds uid to the connection identified by cid and returns the replaced connection, if any.
// The caller must hold the write lock.
func (s *Session) bind(cid, uid int64) (network.Conn, error) {
	conn, err := s.conn(Conn, cid)
	if err != nil {
		return nil, err
	}

	if oldUID := conn.UID(); oldUID != 0 {
		if uid == oldUID {
			return nil, nil
		}

		if err := conn.Bind(uid); err != nil {
			return nil, err
		}

		if c, ok := s.users[oldUID]; ok && c == conn {
			delete(s.users, oldUID)
		}
	} else {
		if err := conn.Bind(uid); err != nil {
			return nil, err
		}
	}

	old := s.users[uid]
	s.users[uid] = conn

	return old, nil
}

// Unbind unbinds the user ID uid and returns the ID of the connection it was bound to.
func (s *Session) Unbind(uid int64) (int64, error) {
	s.rw.Lock()
	defer s.rw.Unlock()

	conn, err := s.conn(User, uid)
	if err != nil {
		return 0, err
	}

	if err := conn.Unbind(); err != nil {
		log.Warnf("unbind user failed: cid = %d, uid = %d, err = %v", conn.ID(), uid, err)
	}

	delete(s.users, uid)

	return conn.ID(), nil
}

// LocalIP returns the local IP address of the session identified by target. The kind selects
// whether target is a connection ID or a user ID.
func (s *Session) LocalIP(kind Kind, target int64) (string, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return "", err
	}

	return conn.LocalIP()
}

// LocalAddr returns the local address of the session identified by target. The kind selects whether
// target is a connection ID or a user ID.
func (s *Session) LocalAddr(kind Kind, target int64) (net.Addr, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return nil, err
	}

	return conn.LocalAddr()
}

// RemoteIP returns the remote IP address of the session identified by target. The kind selects
// whether target is a connection ID or a user ID.
func (s *Session) RemoteIP(kind Kind, target int64) (string, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return "", err
	}

	return conn.RemoteIP()
}

// RemoteAddr returns the remote address of the session identified by target. The kind selects
// whether target is a connection ID or a user ID.
func (s *Session) RemoteAddr(kind Kind, target int64) (net.Addr, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return nil, err
	}

	return conn.RemoteAddr()
}

// Close closes the session identified by target. The kind selects whether target is a connection ID
// or a user ID. Pass force as true to close the connection forcibly.
func (s *Session) Close(kind Kind, target int64, force ...bool) error {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return err
	}

	return conn.Close(force...)
}

// Push pushes buf to the session identified by target asynchronously. The kind selects whether
// target is a connection ID or a user ID. When disconnect is true, the connection is closed after
// the push.
func (s *Session) Push(kind Kind, target int64, disconnect bool, buf buffer.Buffer) error {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		buf.Release()
		return err
	}

	if err = conn.Push(buf); err != nil {
		buf.Release()
		return err
	}

	if disconnect {
		if err = conn.Close(); err != nil {
			log.Warnf("close conn failed: cid = %d, uid = %d, err = %v", conn.ID(), conn.UID(), err)
		}
	}

	return nil
}

// Multicast pushes buf to every session in targets asynchronously. The kind selects whether each
// target is a connection ID or a user ID. When disconnect is true, each connection is closed after
// the push. It returns the number of sessions the message was pushed to, or
// [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *Session) Multicast(kind Kind, targets []int64, disconnect bool, buf buffer.Buffer) (int64, error) {
	if len(targets) == 0 {
		buf.Release()
		return 0, nil
	}

	var conns []network.Conn

	s.rw.RLock()
	switch kind {
	case Conn:
		for _, target := range targets {
			if conn, ok := s.conns[target]; ok {
				conns = append(conns, conn)
			}
		}
	case User:
		for _, target := range targets {
			if conn, ok := s.users[target]; ok {
				conns = append(conns, conn)
			}
		}
	default:
		s.rw.RUnlock()
		buf.Release()
		return 0, errors.ErrInvalidSessionKind
	}
	s.rw.RUnlock()

	if len(conns) == 0 {
		buf.Release()
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, buf)
}

// Broadcast pushes buf to every session of the given kind asynchronously. When disconnect is true,
// each connection is closed after the push. It returns the number of sessions the message was
// pushed to, or [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *Session) Broadcast(kind Kind, disconnect bool, buf buffer.Buffer) (int64, error) {
	var conns []network.Conn

	s.rw.RLock()
	switch kind {
	case Conn:
		conns = make([]network.Conn, 0, len(s.conns))
		for _, conn := range s.conns {
			conns = append(conns, conn)
		}
	case User:
		conns = make([]network.Conn, 0, len(s.users))
		for _, conn := range s.users {
			conns = append(conns, conn)
		}
	default:
		s.rw.RUnlock()
		buf.Release()
		return 0, errors.ErrInvalidSessionKind
	}
	s.rw.RUnlock()

	if len(conns) == 0 {
		buf.Release()
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, buf)
}

// Publish pushes buf to every session subscribed to channel asynchronously. When disconnect is
// true, each connection is closed after the push. It returns the number of sessions the message was
// pushed to.
func (s *Session) Publish(channel string, disconnect bool, buf buffer.Buffer) (int64, error) {
	var conns []network.Conn

	s.rw.RLock()
	if channels, ok := s.channels[channel]; ok {
		conns = make([]network.Conn, 0, len(channels))
		for conn := range channels {
			conns = append(conns, conn)
		}
	}
	s.rw.RUnlock()

	if len(conns) == 0 {
		buf.Release()
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, buf)
}

// Subscribe subscribes the sessions in targets to channel. The kind selects whether each target is
// a connection ID or a user ID. It reports [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *Session) Subscribe(kind Kind, targets []int64, channel string) (err error) {
	if len(targets) == 0 {
		return
	}

	s.rw.Lock()
	defer s.rw.Unlock()

	var conns map[int64]network.Conn
	switch kind {
	case Conn:
		conns = s.conns
	case User:
		conns = s.users
	default:
		err = errors.ErrInvalidSessionKind
		return
	}

	for _, target := range targets {
		conn, ok := conns[target]
		if !ok {
			continue
		}

		conn.Attr().Set(channel, struct{}{})

		if channels, ok := s.channels[channel]; ok {
			channels[conn] = struct{}{}
		} else {
			channels = make(map[network.Conn]struct{}, len(targets))
			channels[conn] = struct{}{}
			s.channels[channel] = channels
		}
	}

	return
}

// Unsubscribe unsubscribes the sessions in targets from channel. The kind selects whether each
// target is a connection ID or a user ID. It reports [errors.ErrInvalidSessionKind] for an unknown
// kind.
func (s *Session) Unsubscribe(kind Kind, targets []int64, channel string) (err error) {
	if len(targets) == 0 {
		return
	}

	s.rw.Lock()
	defer s.rw.Unlock()

	var conns map[int64]network.Conn
	switch kind {
	case Conn:
		conns = s.conns
	case User:
		conns = s.users
	default:
		err = errors.ErrInvalidSessionKind
		return
	}

	for _, target := range targets {
		if conn, ok := conns[target]; ok {
			if ok = conn.Attr().Del(channel); ok {
				s.doUnsubscribe(channel, conn)
			}
		}
	}

	return
}

// doUnsubscribe removes conn from the subscribers of channel. The caller must hold the write lock.
func (s *Session) doUnsubscribe(channel string, conn network.Conn) {
	if channels, ok := s.channels[channel]; ok {
		delete(channels, conn)

		if len(channels) == 0 {
			delete(s.channels, channel)
		}
	}
}

// doClearConnAttrs removes conn from every channel it is subscribed to and clears its attributes.
// The caller must hold the write lock.
func (s *Session) doClearConnAttrs(conn network.Conn) {
	if attr := conn.Attr(); attr != nil {
		attr.Visit(func(key, _ any) bool {
			if channel, ok := key.(string); ok {
				s.doUnsubscribe(channel, conn)
			}
			return true
		})

		attr.Clear()
	}
}

// Stat returns the number of sessions of the given kind, or [errors.ErrInvalidSessionKind] for an
// unknown kind.
func (s *Session) Stat(kind Kind) (int64, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	switch kind {
	case Conn:
		return int64(len(s.conns)), nil
	case User:
		return int64(len(s.users)), nil
	default:
		return 0, errors.ErrInvalidSessionKind
	}
}

// doBatchPush pushes buf to every conn in conns asynchronously. When disconnect is true, each
// connection is closed after the push. It returns the number of sessions the message was pushed to.
func (s *Session) doBatchPush(conns []network.Conn, disconnect bool, buf buffer.Buffer) (int64, error) {
	switch n := len(conns); n {
	case 0:
		buf.Release()
		return 0, nil
	case 1:
		if err := conns[0].Push(buf); err != nil {
			buf.Release()
			return 0, err
		}

		if disconnect {
			_ = conns[0].Close()
		}

		return 1, nil
	default:
		var (
			total atomic.Int64
			eg, _ = task.WithContext(context.Background())
		)

		buf.Delay(n)

		for _, conn := range conns {
			eg.Go(func() error {
				if err := conn.Push(buf); err != nil {
					buf.Release()
					return err
				}

				total.Add(1)

				if disconnect {
					if err := conn.Close(); err != nil {
						log.Warnf("close conn failed: cid = %d, uid = %d, err = %v", conn.ID(), conn.UID(), err)
					}
				}

				return nil
			})
		}

		err := eg.Wait()
		num := total.Load()

		if err != nil {
			if num == 0 {
				return 0, err
			} else {
				log.Warnf("batch push partial failed: total = %d, err = %v", num, err)
			}
		}

		return num, nil
	}
}

// conn returns the connection of the session identified by target. The kind selects whether target
// is a connection ID or a user ID.
func (s *Session) conn(kind Kind, target int64) (network.Conn, error) {
	switch kind {
	case Conn:
		conn, ok := s.conns[target]
		if !ok {
			return nil, errors.ErrNotFoundSession
		}
		return conn, nil
	case User:
		conn, ok := s.users[target]
		if !ok {
			return nil, errors.ErrNotFoundSession
		}
		return conn, nil
	default:
		return nil, errors.ErrInvalidSessionKind
	}
}
