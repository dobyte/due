package gate

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/task"
)

// defaultSessionShardNum is the number of session table shards (a power of two, so that the shard
// is selected with a bit operation).
const defaultSessionShardNum = 256

// sessionShard is a shard of the connection or user session table.
//
// Shards are selected with the low bits of the connection ID or user ID; the struct is padded to a
// 64-byte cache line to avoid false sharing between adjacent shards.
type sessionShard struct {
	rw    sync.RWMutex
	conns map[int64]network.Conn
	_     [32]byte
}

// load returns the connection stored under id.
func (s *sessionShard) load(id int64) (network.Conn, bool) {
	s.rw.RLock()
	conn, ok := s.conns[id]
	s.rw.RUnlock()
	return conn, ok
}

// has reports whether id exists.
func (s *sessionShard) has(id int64) bool {
	s.rw.RLock()
	_, ok := s.conns[id]
	s.rw.RUnlock()
	return ok
}

// store stores conn under id.
func (s *sessionShard) store(id int64, conn network.Conn) {
	s.rw.Lock()
	s.conns[id] = conn
	s.rw.Unlock()
}

// remove deletes id and returns the removed connection.
func (s *sessionShard) remove(id int64, conn ...network.Conn) (network.Conn, bool) {
	s.rw.Lock()

	c, ok := s.conns[id]
	if ok && len(conn) > 0 {
		ok = c == conn[0]
	}

	if ok {
		delete(s.conns, id)
	}

	s.rw.Unlock()

	return c, ok
}

// count returns the number of stored connections.
func (s *sessionShard) count() int {
	s.rw.RLock()
	count := len(s.conns)
	s.rw.RUnlock()
	return count
}

// unbind removes uid after unbinding its connection. A zero cid accepts any connection.
func (s *sessionShard) unbind(uid, cid int64) (network.Conn, bool) {
	s.rw.Lock()
	conn, ok := s.conns[uid]
	if !ok || (cid != 0 && conn.ID() != cid) {
		s.rw.Unlock()
		return nil, false
	}

	if err := conn.Unbind(); err != nil {
		log.Warnf("unbind user failed: cid = %d, uid = %d, err = %v", conn.ID(), uid, err)
	}

	delete(s.conns, uid)
	s.rw.Unlock()

	return conn, true
}

// session manages the connection and user sessions of a node along with their channel
// subscriptions.
//
// Lock ordering is connection shard -> user shard -> channel table. Operations that need multiple
// user shards acquire them in ascending shard index order.
type session struct {
	conns      [defaultSessionShardNum]sessionShard // Connection sessions, sharded by connection ID
	users      [defaultSessionShardNum]sessionShard // User sessions, sharded by user ID
	channelsMu sync.RWMutex                         // Protects the channel subscription table
	channels   map[string]map[network.Conn]struct{} // Channel subscriptions, keyed by channel name
}

// newSession creates a session instance.
func newSession() *session {
	s := &session{
		channels: make(map[string]map[network.Conn]struct{}),
	}

	for i := range s.conns {
		s.conns[i].conns = make(map[int64]network.Conn)
	}

	for i := range s.users {
		s.users[i].conns = make(map[int64]network.Conn)
	}

	return s
}

// shardIndex returns the shard index for the given ID.
func shardIndex(id int64) int {
	return int(uint64(id) & (defaultSessionShardNum - 1))
}

// loadConnShard returns the connection shard of the given connection ID.
func (s *session) loadConnShard(cid int64) *sessionShard {
	return &s.conns[shardIndex(cid)]
}

// loadUserShard returns the user shard of the given user ID.
func (s *session) loadUserShard(uid int64) *sessionShard {
	return &s.users[shardIndex(uid)]
}

// lockUserShards locks the shards of a and b in a deterministic order, returning the locked shards
// in lock order. When both IDs select the same shard, the second shard is nil and the shard is
// locked only once.
func (s *session) lockUserShards(a, b int64) (first, second *sessionShard) {
	ai, bi := shardIndex(a), shardIndex(b)

	if ai == bi {
		shard := &s.users[ai]
		shard.rw.Lock()
		return shard, nil
	}

	if ai < bi {
		first, second = &s.users[ai], &s.users[bi]
	} else {
		first, second = &s.users[bi], &s.users[ai]
	}

	first.rw.Lock()
	second.rw.Lock()

	return
}

// unlockUserShards unlocks the shards acquired by lockUserShards.
func unlockUserShards(first, second *sessionShard) {
	if second != nil {
		second.rw.Unlock()
	}

	first.rw.Unlock()
}

// addConn adds conn to the connection sessions.
func (s *session) addConn(conn network.Conn) {
	cid := conn.ID()
	s.loadConnShard(cid).store(cid, conn)
}

// remConn removes conn from the connection sessions. It reports whether conn was registered and
// has been removed.
//
// The session tables are sharded, so removing a connection only blocks the shard of its CID and
// UID. The channel cleanup runs after the shard locks are released: a concurrent subscribe holds
// the CID shard lock until its channel subscription is recorded, so it either completes before the
// removal (and its channels are cleaned up here) or observes the removed connection.
func (s *session) remConn(conn network.Conn) bool {
	cid := conn.ID()
	if _, ok := s.loadConnShard(cid).remove(cid); !ok {
		return false
	}

	if uid := conn.UID(); uid != 0 {
		s.loadUserShard(uid).remove(uid, conn)
	}

	s.doClearConnAttrs(conn)

	return true
}

// has reports whether the session identified by target exists. The kind selects whether target is
// a connection ID or a user ID. It reports [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *session) has(kind cluster.Session, target int64) (bool, error) {
	switch kind {
	case cluster.Conn:
		return s.loadConnShard(target).has(target), nil
	case cluster.User:
		return s.loadUserShard(target).has(target), nil
	default:
		return false, errors.ErrInvalidSessionKind
	}
}

// uid returns the user ID currently attached to cid, or zero if it is unbound.
func (s *session) uid(cid int64) (int64, error) {
	conn, ok := s.loadConnShard(cid).load(cid)
	if !ok {
		return 0, errors.ErrNotFoundSession
	}

	return conn.UID(), nil
}

// isBound reports whether cid is registered and is the current connection bound to uid.
func (s *session) isBound(cid, uid int64) bool {
	connShard := s.loadConnShard(cid)
	connShard.rw.RLock()
	conn, ok := connShard.conns[cid]
	if !ok || conn.UID() != uid {
		connShard.rw.RUnlock()
		return false
	}

	userShard := s.loadUserShard(uid)
	userShard.rw.RLock()
	bound := userShard.conns[uid] == conn
	userShard.rw.RUnlock()

	connShard.rw.RUnlock()

	return bound
}

// bind binds the user ID uid to the connection identified by cid. When uid is already bound to
// another connection, that connection is closed and replaced.
func (s *session) bind(cid, uid int64) error {
	connShard := s.loadConnShard(cid)
	connShard.rw.Lock()

	conn, ok := connShard.conns[cid]
	if !ok {
		connShard.rw.Unlock()
		return errors.ErrNotFoundSession
	}

	oldUID := conn.UID()
	if oldUID == uid {
		connShard.rw.Unlock()
		return nil
	}

	var (
		first, second *sessionShard
		old           network.Conn
	)

	if oldUID != 0 {
		first, second = s.lockUserShards(oldUID, uid)
	} else {
		first = s.loadUserShard(uid)
		first.rw.Lock()
	}

	if err := conn.Bind(uid); err != nil {
		unlockUserShards(first, second)
		connShard.rw.Unlock()
		return err
	}

	if oldUID != 0 {
		oldShard := s.loadUserShard(oldUID)
		if c, ok := oldShard.conns[oldUID]; ok && c == conn {
			delete(oldShard.conns, oldUID)
		}
	}

	newShard := s.loadUserShard(uid)
	old = newShard.conns[uid]
	newShard.conns[uid] = conn

	unlockUserShards(first, second)
	connShard.rw.Unlock()

	if old != nil {
		if err := old.Close(true); err != nil {
			log.Warnf("close conn failed: cid = %d, uid = %d, err = %v", cid, uid, err)
		}
	}

	return nil
}

// remove unbinds the user ID uid and returns the ID of the connection it was bound to.
func (s *session) remove(uid int64) (int64, error) {
	conn, ok := s.loadUserShard(uid).unbind(uid, 0)
	if !ok {
		return 0, errors.ErrNotFoundSession
	}

	return conn.ID(), nil
}

// unbind unbinds uid only if its current connection is cid. A removed or replaced binding
// returns ErrNotFoundSession without changing the current user connection.
func (s *session) unbind(cid, uid int64) error {
	if cid == 0 {
		return errors.ErrNotFoundSession
	}

	if _, ok := s.loadUserShard(uid).unbind(uid, cid); !ok {
		return errors.ErrNotFoundSession
	}

	return nil
}

// localIP returns the local IP address of the session identified by target. The kind selects
// whether target is a connection ID or a user ID.
func (s *session) localIP(kind cluster.Session, target int64) (string, error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return "", err
	}

	return conn.LocalIP()
}

// localAddr returns the local address of the session identified by target. The kind selects
// whether target is a connection ID or a user ID.
func (s *session) localAddr(kind cluster.Session, target int64) (net.Addr, error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return nil, err
	}

	return conn.LocalAddr()
}

// remoteIP returns the remote IP address of the session identified by target. The kind selects
// whether target is a connection ID or a user ID.
func (s *session) remoteIP(kind cluster.Session, target int64) (string, error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return "", err
	}

	return conn.RemoteIP()
}

// remoteAddr returns the remote address of the session identified by target. The kind selects
// whether target is a connection ID or a user ID.
func (s *session) remoteAddr(kind cluster.Session, target int64) (net.Addr, error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return nil, err
	}

	return conn.RemoteAddr()
}

// close closes the session identified by target. The kind selects whether target is a connection
// ID or a user ID. Pass force as true to close the connection forcibly.
func (s *session) close(kind cluster.Session, target int64, force ...bool) error {
	conn, err := s.conn(kind, target)
	if err != nil {
		return err
	}

	return conn.Close(force...)
}

// push pushes buf to the session identified by target asynchronously. The kind selects whether
// target is a connection ID or a user ID. When disconnect is true, the connection is closed after
// the push.
func (s *session) push(kind cluster.Session, target int64, disconnect bool, buf buffer.Buffer) error {
	conn, err := s.conn(kind, target)
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

// multicast pushes buf to every session in targets asynchronously. The kind selects whether each
// target is a connection ID or a user ID. When disconnect is true, each connection is closed after
// the push. It returns the number of sessions the message was pushed to, or
// [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *session) multicast(kind cluster.Session, targets []int64, disconnect bool, buf buffer.Buffer) (int64, error) {
	if len(targets) == 0 {
		buf.Release()
		return 0, nil
	}

	switch kind {
	case cluster.Conn, cluster.User:
	default:
		buf.Release()
		return 0, errors.ErrInvalidSessionKind
	}

	conns := make([]network.Conn, 0, len(targets))

	for _, target := range targets {
		if conn, err := s.conn(kind, target); err == nil {
			conns = append(conns, conn)
		}
	}

	if len(conns) == 0 {
		buf.Release()
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, buf)
}

// broadcast pushes buf to every session of the given kind asynchronously. When disconnect is true,
// each connection is closed after the push. It returns the number of sessions the message was
// pushed to, or [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *session) broadcast(kind cluster.Session, disconnect bool, buf buffer.Buffer) (int64, error) {
	var conns []network.Conn

	switch kind {
	case cluster.Conn:
		total := 0
		for i := range s.conns {
			s.conns[i].rw.RLock()
			total += len(s.conns[i].conns)
		}
		conns = make([]network.Conn, 0, total)
		for i := range s.conns {
			for _, conn := range s.conns[i].conns {
				conns = append(conns, conn)
			}
		}
		for i := range s.conns {
			s.conns[i].rw.RUnlock()
		}
	case cluster.User:
		total := 0
		for i := range s.users {
			s.users[i].rw.RLock()
			total += len(s.users[i].conns)
		}
		conns = make([]network.Conn, 0, total)
		for i := range s.users {
			for _, conn := range s.users[i].conns {
				conns = append(conns, conn)
			}
		}
		for i := range s.users {
			s.users[i].rw.RUnlock()
		}
	default:
		buf.Release()
		return 0, errors.ErrInvalidSessionKind
	}

	if len(conns) == 0 {
		buf.Release()
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, buf)
}

// publish pushes buf to every session subscribed to channel asynchronously. When disconnect is
// true, each connection is closed after the push. It returns the number of sessions the message was
// pushed to.
func (s *session) publish(channel string, disconnect bool, buf buffer.Buffer) (int64, error) {
	var conns []network.Conn

	s.channelsMu.RLock()
	if channels, ok := s.channels[channel]; ok {
		conns = make([]network.Conn, 0, len(channels))
		for conn := range channels {
			conns = append(conns, conn)
		}
	}
	s.channelsMu.RUnlock()

	if len(conns) == 0 {
		buf.Release()
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, buf)
}

// subscribe subscribes the sessions in targets to channel. The kind selects whether each target is
// a connection ID or a user ID. It reports [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *session) subscribe(kind cluster.Session, targets []int64, channel string) error {
	if len(targets) == 0 {
		return nil
	}

	switch kind {
	case cluster.Conn, cluster.User:
	default:
		return errors.ErrInvalidSessionKind
	}

	for _, target := range targets {
		switch kind {
		case cluster.Conn:
			shard := s.loadConnShard(target)
			shard.rw.Lock()
			if conn, ok := shard.conns[target]; ok {
				s.addSubscription(conn, channel)
			}
			shard.rw.Unlock()
		case cluster.User:
			shard := s.loadUserShard(target)
			shard.rw.Lock()
			if conn, ok := shard.conns[target]; ok {
				s.addSubscription(conn, channel)
			}
			shard.rw.Unlock()
		}
	}

	return nil
}

// unsubscribe unsubscribes the sessions in targets from channel. The kind selects whether each
// target is a connection ID or a user ID. It reports [errors.ErrInvalidSessionKind] for an unknown
// kind.
func (s *session) unsubscribe(kind cluster.Session, targets []int64, channel string) error {
	if len(targets) == 0 {
		return nil
	}

	switch kind {
	case cluster.Conn, cluster.User:
	default:
		return errors.ErrInvalidSessionKind
	}

	for _, target := range targets {
		switch kind {
		case cluster.Conn:
			shard := s.loadConnShard(target)
			shard.rw.Lock()
			if conn, ok := shard.conns[target]; ok {
				if conn.Attr().Del(channel) {
					s.removeSubscription(channel, conn)
				}
			}
			shard.rw.Unlock()
		case cluster.User:
			shard := s.loadUserShard(target)
			shard.rw.Lock()
			if conn, ok := shard.conns[target]; ok {
				if conn.Attr().Del(channel) {
					s.removeSubscription(channel, conn)
				}
			}
			shard.rw.Unlock()
		}
	}

	return nil
}

// addSubscription records channel as a subscription of conn. The caller must hold the connection or
// user shard lock, which keeps conn from being removed and serializes subscription changes for the
// shard.
func (s *session) addSubscription(conn network.Conn, channel string) {
	conn.Attr().Set(channel, struct{}{})

	s.channelsMu.Lock()
	if channels, ok := s.channels[channel]; ok {
		channels[conn] = struct{}{}
	} else {
		channels = make(map[network.Conn]struct{}, 1)
		channels[conn] = struct{}{}
		s.channels[channel] = channels
	}
	s.channelsMu.Unlock()
}

// removeSubscription removes conn from the subscribers of channel.
func (s *session) removeSubscription(channel string, conn network.Conn) {
	s.channelsMu.Lock()
	s.doRemoveSubscription(channel, conn)
	s.channelsMu.Unlock()
}

// doRemoveSubscription removes conn from the subscribers of channel while the caller holds the
// channel lock.
func (s *session) doRemoveSubscription(channel string, conn network.Conn) {
	if channels, ok := s.channels[channel]; ok {
		delete(channels, conn)

		if len(channels) == 0 {
			delete(s.channels, channel)
		}
	}
}

// doClearConnAttrs removes conn from every channel it is subscribed to and clears its attributes.
// It runs after the session tables have released conn, so it no longer blocks connection and user
// lookups for other sessions.
func (s *session) doClearConnAttrs(conn network.Conn) {
	attr := conn.Attr()
	if attr == nil {
		return
	}

	var channels []string

	attr.Visit(func(key, _ any) bool {
		if channel, ok := key.(string); ok {
			channels = append(channels, channel)
		}
		return true
	})

	attr.Clear()

	if len(channels) == 0 {
		return
	}

	s.channelsMu.Lock()
	for _, channel := range channels {
		s.doRemoveSubscription(channel, conn)
	}
	s.channelsMu.Unlock()
}

// stat returns the number of sessions of the given kind, or [errors.ErrInvalidSessionKind] for an
// unknown kind.
func (s *session) stat(kind cluster.Session) (int64, error) {
	switch kind {
	case cluster.Conn:
		var total int64
		for i := range s.conns {
			total += int64(s.conns[i].count())
		}
		return total, nil
	case cluster.User:
		var total int64
		for i := range s.users {
			total += int64(s.users[i].count())
		}
		return total, nil
	default:
		return 0, errors.ErrInvalidSessionKind
	}
}

// doBatchPush pushes buf to every conn in conns asynchronously. When disconnect is true, each
// connection is closed after the push. It returns the number of sessions the message was pushed to.
func (s *session) doBatchPush(conns []network.Conn, disconnect bool, buf buffer.Buffer) (int64, error) {
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
func (s *session) conn(kind cluster.Session, target int64) (network.Conn, error) {
	switch kind {
	case cluster.Conn:
		if conn, ok := s.loadConnShard(target).load(target); ok {
			return conn, nil
		}
	case cluster.User:
		if conn, ok := s.loadUserShard(target).load(target); ok {
			return conn, nil
		}
	default:
		return nil, errors.ErrInvalidSessionKind
	}

	return nil, errors.ErrNotFoundSession
}
