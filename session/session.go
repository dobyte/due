package session

import (
	"fmt"
	"github.com/orcaman/concurrent-map/v2"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network"
	"net"
	"sync/atomic"
	"time"
)

var (
	ErrNotFoundSession    = errors.New("not found session")
	ErrInvalidSessionKind = errors.New("invalid session kind")
)

const (
	Conn Kind = iota + 1 // 连接SESSION
	User                 // 用户SESSION
)

type Kind int

type Session struct {
	conns cmap.ConcurrentMap[string, network.Conn] // 连接会话（连接ID -> network.Conn）
	users cmap.ConcurrentMap[string, network.Conn] // 用户会话（用户ID -> network.Conn）
}

func NewSession() *Session {
	return &Session{
		conns: cmap.New[network.Conn](),
		users: cmap.New[network.Conn](),
	}
}

// AddConn 添加连接
func (s *Session) AddConn(conn network.Conn) {
	cid, uid := conn.ID(), conn.UID()
	s.conns.Set(fmt.Sprintf("%+v", cid), conn)
	if uid != 0 {
		s.users.Set(fmt.Sprintf("%+v", uid), conn)
	}
}

// RemConn 移除连接
func (s *Session) RemConn(conn network.Conn) {
	cid, uid := conn.ID(), conn.UID()
	s.conns.Remove(fmt.Sprintf("%+v", cid))
	if uid != 0 {
		s.users.Remove(fmt.Sprintf("%+v", uid))
	}
}

// RemConnByCid 移除连接
func (s *Session) RemConnByCid(cid int64) {
	s.conns.Remove(fmt.Sprintf("%+v", cid))
	log.Infof("RemConnByCid conns add cid:%+v", cid)
}

// Has 是否存在会话
func (s *Session) Has(kind Kind, target int64) (ok bool, err error) {
	switch kind {
	case Conn:
		_, ok = s.conns.Get(fmt.Sprintf("%+v", target))
	case User:
		_, ok = s.users.Get(fmt.Sprintf("%+v", target))
	default:
		err = ErrInvalidSessionKind
	}

	return
}

// ID 获取conn的ID
func (s *Session) ID(kind Kind, target int64) (id int64, err error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return 0, err
	}
	return conn.ID(), nil
}

// Bind 绑定用户ID
func (s *Session) Bind(cid, uid int64) error {

	conn, err := s.conn(Conn, cid)
	if err != nil {
		return err
	}

	if oldUID := conn.UID(); oldUID != 0 {
		if uid == oldUID {
			return nil
		}
		s.users.Remove(fmt.Sprintf("%+v", oldUID))
		log.Infof("Bind users remove cid:%+v,uid:%+v,oldUid:%+v", cid, uid, oldUID)
	}

	if oldConn, _ := s.conn(User, uid); oldConn != nil {
		oldConn.Unbind()
	}

	(conn).Bind(uid)
	s.users.Set(fmt.Sprintf("%+v", uid), conn)

	return nil
}

// Unbind 解绑用户ID
func (s *Session) Unbind(uid int64) (int64, error) {

	conn, err := s.conn(User, uid)
	if err != nil {
		return 0, err
	}

	(conn).Unbind()
	s.users.Remove(fmt.Sprintf("%+v", uid))
	log.Infof("Bind users remove uid:%+v", uid)

	return (conn).ID(), nil
}

// LocalIP 获取本地IP
func (s *Session) LocalIP(kind Kind, target int64) (string, error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return "", err
	}

	return (conn).LocalIP()
}

// LocalAddr 获取本地地址
func (s *Session) LocalAddr(kind Kind, target int64) (net.Addr, error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return nil, err
	}

	return conn.LocalAddr()
}

// RemoteIP 获取远端IP
func (s *Session) RemoteIP(kind Kind, target int64) (string, error) {

	conn, err := s.conn(kind, target)
	if err != nil {
		return "", err
	}

	return conn.RemoteIP()
}

// RemoteAddr 获取远端地址
func (s *Session) RemoteAddr(kind Kind, target int64) (net.Addr, error) {
	conn, err := s.conn(kind, target)
	if err != nil {
		return nil, err
	}

	return conn.RemoteAddr()
}

// Close 关闭会话
func (s *Session) Close(kind Kind, target int64, isForce ...bool) error {
	conn, err := s.conn(kind, target)

	if err != nil {
		return err
	}

	return conn.Close(isForce...)
}

// Send 发送消息（同步）
func (s *Session) Send(kind Kind, target int64, msg []byte, msgType ...int) error {
	conn, err := s.conn(kind, target)
	if err != nil {
		return err
	}

	return conn.Send(msg, msgType...)
}

// Push 推送消息（异步）
func (s *Session) Push(kind Kind, target int64, msg []byte, msgType ...int) error {
	conn, err := s.conn(kind, target)
	if err != nil {
		return err
	}

	return conn.Push(msg, msgType...)
}

// Multicast 推送组播消息（异步）
func (s *Session) Multicast(kind Kind, targets []int64, msg []byte, msgType ...int) (n int64, err error) {
	if len(targets) == 0 {
		return
	}
	start := time.Now()
	for _, target := range targets {
		if conn, err := s.conn(kind, target); err == nil {
			//这里消息可能会乱序，有问题@yimin
			go func(c network.Conn) {
				if err := c.Push(msg, msgType...); err != nil {
					//log.Warnf("cid:%+v,uid:%+v push message to client failed.err:%+v", c.ID(), c.UID(), err)
				}
			}(conn)
			atomic.AddInt64(&n, 1)
		} else {
			continue
		}

	}
	spent := time.Now().UnixMilli() - start.UnixMilli()

	if spent > 10 {
		log.Debugf("total:%+v,targets count:%+v,kind:%+v,multicast time:%+v ms", n, len(targets), kind, spent)
	}

	return
}

// Broadcast 推送广播消息（异步）
func (s *Session) Broadcast(kind Kind, msg []byte, msgType ...int) (n int64, err error) {
	switch kind {
	case Conn:
		s.conns.IterCb(func(key string, conn network.Conn) {
			if conn.Push(msg, msgType...) == nil {
				n++
			}
		})
	case User:
		s.users.IterCb(func(key string, conn network.Conn) {
			if conn.Push(msg, msgType...) == nil {
				n++
			}
		})
	default:
		err = ErrInvalidSessionKind
		return
	}

	return
}

// Stat 统计会话总数
func (s *Session) Stat(kind Kind) (int64, error) {
	switch kind {
	case Conn:
		var len int64
		s.conns.IterCb(func(key string, conn network.Conn) {
			atomic.AddInt64(&len, 1)
		})
		return len, nil
	case User:
		var len int64
		s.users.IterCb(func(key string, conn network.Conn) {
			atomic.AddInt64(&len, 1)
		})
		return len, nil
	default:
		return 0, ErrInvalidSessionKind
	}
}

// IdsStat 获取所有ID
func (s *Session) IdsStat(kind Kind) ([]string, error) {
	var idList []string
	switch kind {
	case Conn:
		s.users.IterCb(func(key string, conn network.Conn) {
			idList = append(idList, key)
		})
		return idList, nil
	case User:
		s.users.IterCb(func(key string, conn network.Conn) {
			idList = append(idList, key)
		})
		return idList, nil
	default:
		return nil, ErrInvalidSessionKind
	}
}

func (s *Session) Conn(kind Kind, target int64) (network.Conn, error) {
	return s.conn(kind, target)
}

// 获取会话
func (s *Session) conn(kind Kind, target int64) (network.Conn, error) {
	start := time.Now()
	switch kind {
	case Conn:
		value, ok := s.conns.Get(fmt.Sprintf("%+v", target))
		if !ok {
			return nil, ErrNotFoundSession
		}
		conn := value.(network.Conn)
		spent := time.Now().UnixMilli() - start.UnixMilli()

		if spent > 1 {
			log.Debugf("session get conn. kind:%+v,multicast time:%+v ms", kind, spent)

		}
		return conn, nil
	case User:
		value, ok := s.users.Get(fmt.Sprintf("%+v", target))
		if !ok {
			return nil, ErrNotFoundSession
		}
		conn := value.(network.Conn)
		spent := time.Now().UnixMilli() - start.UnixMilli()

		if spent > 1 {
			log.Debugf("session get conn. kind:%+v,multicast time:%+v ms", kind, spent)

		}
		return conn, nil
	default:
		return nil, ErrInvalidSessionKind
	}
}
