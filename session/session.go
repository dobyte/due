package session

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/task"
)

const (
	Conn Kind = iota + 1 // 连接SESSION
	User                 // 用户SESSION
)

type Kind int

// String 获取会话类型名称
// @return @1 string 会话类型名称
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

// NewSession 创建一个会话实例
// @return @1 *Session 会话实例
func NewSession() *Session {
	return &Session{
		conns:    make(map[int64]network.Conn),
		users:    make(map[int64]network.Conn),
		channels: make(map[string]map[network.Conn]struct{}),
	}
}

// AddConn 添加连接
// @param conn network.Conn 连接对象
func (s *Session) AddConn(conn network.Conn) {
	s.rw.Lock()
	s.conns[conn.ID()] = conn
	s.rw.Unlock()
}

// RemConn 移除连接
// @param conn network.Conn 连接对象
func (s *Session) RemConn(conn network.Conn) {
	s.rw.Lock()
	defer s.rw.Unlock()

	cid, uid := conn.ID(), conn.UID()

	delete(s.conns, cid)

	if uid != 0 {
		if c, ok := s.users[uid]; ok && c == conn {
			delete(s.users, uid)
		}
	}

	s.doClearConnAttrs(conn)
}

// Has 判断会话是否存在
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @return @1 bool 是否存在
// @return @2 error 错误信息
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
// @param cid int64 连接ID
// @param uid int64 用户ID
// @return @1 error 错误信息
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

// bind 执行绑定用户ID操作
// @param cid int64 连接ID
// @param uid int64 用户ID
// @return @1 network.Conn 被替换的旧连接
// @return @2 error 错误信息
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

// Unbind 解绑用户ID
// @param uid int64 用户ID
// @return @1 int64 连接ID
// @return @2 error 错误信息
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

// LocalIP 获取本地IP
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @return @1 string 本地IP地址
// @return @2 error 错误信息
func (s *Session) LocalIP(kind Kind, target int64) (string, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return "", err
	}

	return conn.LocalIP()
}

// LocalAddr 获取本地地址
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @return @1 net.Addr 本地地址
// @return @2 error 错误信息
func (s *Session) LocalAddr(kind Kind, target int64) (net.Addr, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return nil, err
	}

	return conn.LocalAddr()
}

// RemoteIP 获取远端IP
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @return @1 string 远端IP地址
// @return @2 error 错误信息
func (s *Session) RemoteIP(kind Kind, target int64) (string, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return "", err
	}

	return conn.RemoteIP()
}

// RemoteAddr 获取远端地址
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @return @1 net.Addr 远端地址
// @return @2 error 错误信息
func (s *Session) RemoteAddr(kind Kind, target int64) (net.Addr, error) {
	s.rw.RLock()
	conn, err := s.conn(kind, target)
	s.rw.RUnlock()

	if err != nil {
		return nil, err
	}

	return conn.RemoteAddr()
}

// Close 关闭会话
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @param force ...bool 是否强制关闭
// @return @1 error 错误信息
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
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @param message []byte 消息内容
// @return @1 error 错误信息
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
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @param disconnect bool 推送后是否断开连接
// @param message []byte 消息内容
// @return @1 error 错误信息
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
// @param kind Kind 会话类型
// @param targets []int64 会话目标列表（连接ID或用户ID）
// @param disconnect bool 推送后是否断开连接
// @param message []byte 消息内容
// @return @1 int64 成功推送的数量
// @return @2 error 错误信息
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
// @param kind Kind 会话类型
// @param disconnect bool 推送后是否断开连接
// @param message []byte 消息内容
// @return @1 int64 成功推送的数量
// @return @2 error 错误信息
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
// @param channel string 频道名称
// @param disconnect bool 推送后是否断开连接
// @param message []byte 消息内容
// @return @1 int64 成功推送的数量
// @return @2 error 错误信息
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
// @param kind Kind 会话类型
// @param targets []int64 会话目标列表（连接ID或用户ID）
// @param channel string 频道名称
// @return @1 error 错误信息
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
// @param kind Kind 会话类型
// @param targets []int64 会话目标列表（连接ID或用户ID）
// @param channel string 频道名称
// @return @1 error 错误信息
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

// doUnsubscribe 取消订阅频道
// @param channel string 频道名称
// @param conn network.Conn 连接对象
func (s *Session) doUnsubscribe(channel string, conn network.Conn) {
	if channels, ok := s.channels[channel]; ok {
		delete(channels, conn)

		if len(channels) == 0 {
			delete(s.channels, channel)
		}
	}
}

// doClearConnAttrs 清除连接属性
// @param conn network.Conn 连接对象
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

// Stat 统计会话总数
// @param kind Kind 会话类型
// @return @1 int64 会话数量
// @return @2 error 错误信息
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

// doBatchPush 批量推送消息（异步）
// @param conns []network.Conn 连接对象列表
// @param disconnect bool 推送后是否断开连接
// @param message []byte 消息内容
// @return @1 int64 成功推送的数量
// @return @2 error 错误信息
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
			total atomic.Int64
			eg, _ = task.WithContext(context.Background())
		)

		for _, conn := range conns {
			eg.Go(func() error {
				if err := conn.Push(message); err != nil {
					return err
				}

				total.Add(1)

				if disconnect {
					_ = conn.Close()
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

// conn 获取会话连接
// @param kind Kind 会话类型
// @param target int64 会话目标（连接ID或用户ID）
// @return @1 network.Conn 连接对象
// @return @2 error 错误信息
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
