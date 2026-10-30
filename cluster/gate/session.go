package gate

import (
	"context"
	"hash/maphash"
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

// defaultChannelShardNum is the number of channel subscription table shards (a power of two, so
// that the shard is selected with a bit operation).
const defaultChannelShardNum = 256

// defaultBatchPushConcurrency is the maximum number of concurrent pushes in a batch. A push can
// block while the write queue of a slow connection is full, so the fan-out is bounded to keep one
// slow connection from delaying the rest of the batch without spawning a goroutine per session.
const defaultBatchPushConcurrency = 64

// channelHashSeed hashes channel names to select a channel shard.
var channelHashSeed = maphash.MakeSeed()

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

// store stores conn under id. It reports whether the ID was absent before.
func (s *sessionShard) store(id int64, conn network.Conn) (added bool) {
	s.rw.Lock()
	_, ok := s.conns[id]
	s.conns[id] = conn
	s.rw.Unlock()

	return !ok
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

// channelShard is a shard of the channel subscription table.
type channelShard struct {
	rw       sync.RWMutex
	channels map[string]*channelSubscribers
	_        [32]byte
}

// channelSubscribers holds the subscribers of one channel.
//
// The members map accepts subscription changes, while snapshot is an immutable connection list
// read by publish. Every change invalidates the snapshot; it is rebuilt lazily on the next publish.
type channelSubscribers struct {
	mu       sync.Mutex
	members  map[network.Conn]struct{}
	snapshot atomic.Pointer[[]network.Conn]
}

// add adds conn to the subscriber set.
func (c *channelSubscribers) add(conn network.Conn) {
	c.mu.Lock()
	if _, ok := c.members[conn]; !ok {
		c.members[conn] = struct{}{}
		c.snapshot.Store(nil)
	}
	c.mu.Unlock()
}

// remove removes conn from the subscriber set. It reports whether conn was removed and whether the
// set is empty afterwards.
func (c *channelSubscribers) remove(conn network.Conn) (removed, empty bool) {
	c.mu.Lock()
	if _, ok := c.members[conn]; ok {
		delete(c.members, conn)
		c.snapshot.Store(nil)
		removed = true
	}
	empty = len(c.members) == 0
	c.mu.Unlock()
	return
}

// subscribers returns an immutable snapshot of the subscriber set. The snapshot is rebuilt after
// the set changes and then reused by subsequent publishes.
func (c *channelSubscribers) subscribers() []network.Conn {
	if snapshot := c.snapshot.Load(); snapshot != nil {
		return *snapshot
	}

	c.mu.Lock()
	if snapshot := c.snapshot.Load(); snapshot != nil {
		c.mu.Unlock()
		return *snapshot
	}

	if len(c.members) == 0 {
		c.mu.Unlock()
		return nil
	}

	conns := make([]network.Conn, 0, len(c.members))
	for conn := range c.members {
		conns = append(conns, conn)
	}
	c.snapshot.Store(&conns)
	c.mu.Unlock()

	return conns
}

// session manages the connection and user sessions of a node along with their channel
// subscriptions.
//
// Lock ordering is connection shard -> user shard -> channel shard -> channel subscriber mutex.
// Operations that need multiple user shards acquire them in ascending shard index order.
type session struct {
	conns     [defaultSessionShardNum]sessionShard // Connection sessions, sharded by connection ID
	users     [defaultSessionShardNum]sessionShard // User sessions, sharded by user ID
	channels  [defaultChannelShardNum]channelShard // Channel subscriptions, sharded by channel name
	connCount atomic.Int64                         // Number of connection sessions
	userCount atomic.Int64                         // Number of user sessions
}

// newSession creates a session instance.
func newSession() *session {
	s := &session{}

	for i := range s.conns {
		s.conns[i].conns = make(map[int64]network.Conn)
	}

	for i := range s.users {
		s.users[i].conns = make(map[int64]network.Conn)
	}

	for i := range s.channels {
		s.channels[i].channels = make(map[string]*channelSubscribers)
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

// loadChannelShard returns the channel shard of the given channel name.
func (s *session) loadChannelShard(channel string) *channelShard {
	return &s.channels[int(maphash.String(channelHashSeed, channel)&(defaultChannelShardNum-1))]
}

// loadChannel returns the subscribers of channel, or nil when the channel has no subscribers.
func (s *session) loadChannel(channel string) *channelSubscribers {
	shard := s.loadChannelShard(channel)
	shard.rw.RLock()
	subscribers := shard.channels[channel]
	shard.rw.RUnlock()
	return subscribers
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
	if s.loadConnShard(cid).store(cid, conn) {
		s.connCount.Add(1)
	}
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
	s.connCount.Add(-1)

	if uid := conn.UID(); uid != 0 {
		if _, ok := s.loadUserShard(uid).remove(uid, conn); ok {
			s.userCount.Add(-1)
		}
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
			s.userCount.Add(-1)
		}
	}

	newShard := s.loadUserShard(uid)
	old, exists := newShard.conns[uid]
	if !exists {
		s.userCount.Add(1)
	}
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
	s.userCount.Add(-1)

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
	s.userCount.Add(-1)

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

	return s.doBatchPush(disconnect, buf, len(conns), conns)
}

// broadcast pushes buf to every session of the given kind asynchronously. When disconnect is true,
// each connection is closed after the push. It returns the number of sessions the message was
// pushed to, or [errors.ErrInvalidSessionKind] for an unknown kind.
func (s *session) broadcast(kind cluster.Session, disconnect bool, buf buffer.Buffer) (int64, error) {
	var (
		total  int
		conns  [][]network.Conn
		shards []sessionShard
	)

	switch kind {
	case cluster.Conn:
		shards = s.conns[:]
	case cluster.User:
		shards = s.users[:]
	default:
		buf.Release()
		return 0, errors.ErrInvalidSessionKind
	}

	conns = make([][]network.Conn, len(shards))
	for i := range shards {
		shards[i].rw.RLock()
		total += len(shards[i].conns)
		conns[i] = make([]network.Conn, 0, len(shards[i].conns))
		for _, conn := range shards[i].conns {
			conns[i] = append(conns[i], conn)
		}
		shards[i].rw.RUnlock()
	}

	return s.doBatchPush(disconnect, buf, total, conns[:]...)
}

// publish pushes buf to every session subscribed to channel asynchronously. When disconnect is
// true, each connection is closed after the push. It returns the number of sessions the message was
// pushed to.
func (s *session) publish(channel string, disconnect bool, buf buffer.Buffer) (int64, error) {
	subscribers := s.loadChannel(channel)
	if subscribers == nil {
		buf.Release()
		return 0, nil
	}

	conns := subscribers.subscribers()
	if len(conns) == 0 {
		buf.Release()
		return 0, nil
	}

	return s.doBatchPush(disconnect, buf, len(conns), conns)
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

	shard := s.loadChannelShard(channel)
	shard.rw.Lock()
	subscribers := shard.channels[channel]
	if subscribers == nil {
		subscribers = &channelSubscribers{
			members: make(map[network.Conn]struct{}),
		}
		shard.channels[channel] = subscribers
	}
	subscribers.add(conn)
	shard.rw.Unlock()
}

// removeSubscription removes conn from the subscribers of channel.
func (s *session) removeSubscription(channel string, conn network.Conn) {
	shard := s.loadChannelShard(channel)
	shard.rw.Lock()
	subscribers := shard.channels[channel]
	if subscribers == nil {
		shard.rw.Unlock()
		return
	}

	_, empty := subscribers.remove(conn)
	if empty {
		delete(shard.channels, channel)
	}
	shard.rw.Unlock()
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

	for _, channel := range channels {
		s.removeSubscription(channel, conn)
	}
}

// stat returns the number of sessions of the given kind, or [errors.ErrInvalidSessionKind] for an
// unknown kind.
func (s *session) stat(kind cluster.Session) (int64, error) {
	switch kind {
	case cluster.Conn:
		return s.connCount.Load(), nil
	case cluster.User:
		return s.userCount.Load(), nil
	default:
		return 0, errors.ErrInvalidSessionKind
	}
}

// doBatchPush pushes buf to every conn in lists asynchronously. When disconnect is true, each
// connection is closed after the push. It returns the number of sessions the message was pushed to.
//
// The pushes run concurrently with a bounded fan-out: a push may block while the write queue of a
// slow connection is full, and the bound keeps such a connection from delaying the rest of the
// batch without spawning one goroutine per session. A single push runs inline.
func (s *session) doBatchPush(disconnect bool, buf buffer.Buffer, total int, lists ...[]network.Conn) (int64, error) {
	switch total {
	case 0:
		buf.Release()
		return 0, nil
	case 1:
		for _, list := range lists {
			for _, conn := range list {
				if err := conn.Push(buf); err != nil {
					buf.Release()
					return 0, err
				}

				if disconnect {
					if err := conn.Close(); err != nil {
						log.Warnf("close conn failed: cid = %d, uid = %d, err = %v", conn.ID(), conn.UID(), err)
					}
				}

				return 1, nil
			}
		}

		return 0, errors.ErrUnknownError
	default:
		var (
			num   int64
			eg, _ = task.WithContext(context.Background())
		)

		buf.Delay(total)

		for _, list := range lists {
			for _, conn := range list {
				eg.Go(func() error {
					if err := conn.Push(buf); err != nil {
						buf.Release()
						return err
					}

					atomic.AddInt64(&num, 1)

					if disconnect {
						if err := conn.Close(); err != nil {
							log.Warnf("close conn failed: cid = %d, uid = %d, err = %v", conn.ID(), conn.UID(), err)
						}
					}

					return nil
				})
			}
		}

		if err := eg.Wait(); err != nil {
			if num == 0 {
				return 0, err
			}

			log.Warnf("batch push partial failed: total = %d, err = %v", num, err)
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
