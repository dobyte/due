package session

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/task"
)

const (
	Conn Kind = iota + 1 // 连接SESSION
	User                 // 用户SESSION
)

type Kind int

func (k Kind) String() string {
	switch k {
	case Conn:
		return "conn"
	case User:
		return "user"
	}

	return ""
}

type Session struct {
	rw       sync.RWMutex                         // 读写锁
	conns    map[int64]network.Conn               // 连接会话（连接ID -> network.Conn）
	users    map[int64]network.Conn               // 用户会话（用户ID -> network.Conn）
	channels map[string]map[network.Conn]struct{} // 会话频道（频道名 -> [network.Conn --> none]）
}

func NewSession() *Session {
	return &Session{
		conns:    make(map[int64]network.Conn),
		users:    make(map[int64]network.Conn),
		channels: make(map[string]map[network.Conn]struct{}),
	}
}

// AddConn 添加连接
func (s *Session) AddConn(conn network.Conn) {
	s.rw.Lock()
	defer s.rw.Unlock()

	cid, uid := conn.ID(), conn.UID()

	if c, ok := s.conns[cid]; ok && c != conn {
		s.doClearConnAttrs(c)
	}

	s.conns[cid] = conn

	if uid == 0 {
		return
	}

	if c, ok := s.users[uid]; ok && c != conn {
		c.Unbind()
		s.doClearConnAttrs(c)
	}

	s.users[uid] = conn
}

// RemConn 移除连接
func (s *Session) RemConn(conn network.Conn) {
	s.rw.Lock()
	defer s.rw.Unlock()

	cid, uid := conn.ID(), conn.UID()

	delete(s.conns, cid)

	if uid != 0 {
		delete(s.users, uid)
	}

	s.doClearConnAttrs(conn)
}

// Has 是否存在会话
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

// Bind 绑定用户ID
func (s *Session) Bind(cid, uid int64) error {
	s.rw.Lock()
	defer s.rw.Unlock()

	conn, err := s.conn(Conn, cid)
	if err != nil {
		return err
	}

	if oldUID := conn.UID(); oldUID != 0 {
		if uid == oldUID {
			return nil
		}
		delete(s.users, oldUID)
	}

	if oldConn, ok := s.users[uid]; ok {
		oldConn.Unbind()
	}

	conn.Bind(uid)
	s.users[uid] = conn

	return nil
}

// Unbind 解绑用户ID
func (s *Session) Unbind(uid int64) (int64, error) {
	s.rw.Lock()
	defer s.rw.Unlock()

	conn, err := s.conn(User, uid)
	if err != nil {
		return 0, err
	}

	conn.Unbind()
	delete(s.users, uid)

	return conn.ID(), nil
}

// LocalIP 获取本地IP
func (s *Session) LocalIP(kind Kind, target int64) (string, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	conn, err := s.conn(kind, target)
	if err != nil {
		return "", err
	}

	return conn.LocalIP()
}

// LocalAddr 获取本地地址
func (s *Session) LocalAddr(kind Kind, target int64) (net.Addr, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	conn, err := s.conn(kind, target)
	if err != nil {
		return nil, err
	}

	return conn.LocalAddr()
}

// RemoteIP 获取远端IP
func (s *Session) RemoteIP(kind Kind, target int64) (string, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	conn, err := s.conn(kind, target)
	if err != nil {
		return "", err
	}

	return conn.RemoteIP()
}

// RemoteAddr 获取远端地址
func (s *Session) RemoteAddr(kind Kind, target int64) (net.Addr, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	conn, err := s.conn(kind, target)
	if err != nil {
		return nil, err
	}

	return conn.RemoteAddr()
}

// Close 关闭会话
func (s *Session) Close(kind Kind, target int64, force ...bool) error {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return err
	}

	return conn.Close(force...)
}

// Send 发送消息（同步）
func (s *Session) Send(kind Kind, target int64, message []byte) error {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return err
	}

	return conn.Send(message)
}

// Push 推送消息（异步）
func (s *Session) Push(kind Kind, target int64, disconnect bool, message []byte) error {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return err
	}

	if err = conn.Push(message); err != nil {
		return err
	}

	if disconnect {
		return conn.Close()
	} else {
		return nil
	}
}

// Multicast 推送组播消息（异步）
func (s *Session) Multicast(kind Kind, targets []int64, disconnect bool, message []byte) (int64, error) {
	if len(targets) == 0 {
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
		return 0, errors.ErrInvalidSessionKind
	}
	s.rw.RUnlock()

	if len(conns) == 0 {
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, message)
}

// Broadcast 推送广播消息（异步）
func (s *Session) Broadcast(kind Kind, disconnect bool, message []byte) (int64, error) {
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
		return 0, errors.ErrInvalidSessionKind
	}
	s.rw.RUnlock()

	if len(conns) == 0 {
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, message)
}

// Publish 发布频道消息（异步）
func (s *Session) Publish(channel string, disconnect bool, message []byte) (int64, error) {
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
		return 0, nil
	}

	return s.doBatchPush(conns, disconnect, message)
}

// Subscribe 订阅频道
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

// Unsubscribe 取消订阅频道
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

// 取消订阅频道
func (s *Session) doUnsubscribe(channel string, conn network.Conn) {
	if channels, ok := s.channels[channel]; ok {
		delete(channels, conn)

		if len(channels) == 0 {
			delete(s.channels, channel)
		}
	}
}

// 清除连接属性
func (s *Session) doClearConnAttrs(conn network.Conn) {
	conn.Attr().Visit(func(channel, _ any) bool {
		s.doUnsubscribe(channel.(string), conn)

		return true
	})
}

// Stat 统计会话总数
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

// 批量推送消息（异步）
func (s *Session) doBatchPush(conns []network.Conn, disconnect bool, message []byte) (int64, error) {
	switch len(conns) {
	case 0:
		return 0, nil
	case 1:
		if err := conns[0].Push(message); err != nil {
			return 0, err
		}

		if disconnect {
			_ = conns[0].Close()
		}

		return 1, nil
	default:
		var (
			total int64
			eg, _ = task.WithContext(context.Background())
		)

		for _, conn := range conns {
			eg.Go(func() error {
				if err := conn.Push(message); err != nil {
					return err
				}

				atomic.AddInt64(&total, 1)

				if disconnect {
					_ = conn.Close()
				}

				return nil
			})
		}

		if err := eg.Wait(); err != nil && total == 0 {
			return 0, err
		} else {
			return total, nil
		}
	}
}

// 获取会话
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
