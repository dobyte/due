/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/9 19:58
 * @Desc: TODO
 */

package session

import (
	"errors"
	"sync"
)

var (
	ErrNotFoundSession    = errors.New("the session not found in the group")
	ErrInvalidSessionKind = errors.New("invalid session kind")
)

const (
	Conn Kind = iota + 1 // 连接SESSION
	User                 // 用户SESSION
)

type Kind int

type Group struct {
	rw    sync.RWMutex       // 读写锁
	conns map[int64]*Session // 连接会话（连接ID -> *Session）
	users map[int64]*Session // 用户会话（用户ID -> *Session）
}

func NewGroup() *Group {
	return &Group{
		conns: make(map[int64]*Session),
		users: make(map[int64]*Session),
	}
}

// AddSession 添加会话
func (g *Group) AddSession(sess *Session) {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.conns[sess.CID()] = sess
	if uid := sess.UID(); uid > 0 {
		g.users[uid] = sess
	}
	sess.addToGroups(g)
}

// RemSession 移除会话
func (g *Group) RemSession(kind Kind, target int64) (*Session, error) {
	g.rw.Lock()
	defer g.rw.Unlock()

	switch kind {
	case Conn:
		sess, ok := g.conns[target]
		if !ok {
			return nil, ErrNotFoundSession
		}

		if uid := sess.UID(); uid > 0 {
			delete(g.users, uid)
		}
		delete(g.conns, target)
		sess.remFromGroups(g)
		return sess, nil
	case User:
		sess, ok := g.users[target]
		if !ok {
			return nil, ErrNotFoundSession
		}

		delete(g.users, target)
		delete(g.conns, sess.CID())
		sess.remFromGroups(g)
		return sess, nil
	default:
		return nil, ErrInvalidSessionKind
	}
}

// GetSession 获取会话
func (g *Group) GetSession(kind Kind, target int64) (*Session, error) {
	g.rw.RLock()
	defer g.rw.RUnlock()

	return g.getSession(kind, target)
}

// 获取会话
func (g *Group) getSession(kind Kind, target int64) (*Session, error) {
	switch kind {
	case Conn:
		sess, ok := g.conns[target]
		if !ok {
			return nil, ErrNotFoundSession
		}
		return sess, nil
	case User:
		sess, ok := g.users[target]
		if !ok {
			return nil, ErrNotFoundSession
		}
		return sess, nil
	default:
		return nil, ErrInvalidSessionKind
	}
}

// Send 发送消息（同步）
func (g *Group) Send(kind Kind, target int64, msg []byte, typ ...int) error {
	sess, err := g.GetSession(kind, target)
	if err != nil {
		return err
	}

	return sess.Send(msg, typ...)
}

// Push 推送消息（异步）
func (g *Group) Push(kind Kind, target int64, msg []byte, msgType ...int) error {
	sess, err := g.GetSession(kind, target)
	if err != nil {
		return err
	}

	return sess.Push(msg, msgType...)
}

// Multicast 推送组播消息（异步）
func (g *Group) Multicast(kind Kind, targets []int64, msg []byte, msgType ...int) (n int, err error) {
	g.rw.RLock()
	defer g.rw.RUnlock()

	var sessions map[int64]*Session
	switch kind {
	case Conn:
		sessions = g.conns
	case User:
		sessions = g.users
	default:
		err = ErrInvalidSessionKind
		return
	}

	for _, target := range targets {
		session, ok := sessions[target]
		if !ok {
			continue
		}
		if session.Push(msg, msgType...) == nil {
			n++
		}
	}

	return
}

// Broadcast 推送广播消息（异步）
func (g *Group) Broadcast(kind Kind, msg []byte, msgType ...int) (n int, err error) {
	g.rw.RLock()
	defer g.rw.RUnlock()

	var sessions map[int64]*Session
	switch kind {
	case Conn:
		sessions = g.conns
	case User:
		sessions = g.users
	default:
		err = ErrInvalidSessionKind
		return
	}

	for _, session := range sessions {
		if session.Push(msg, msgType...) == nil {
			n++
		}
	}

	return
}

// 添加会话
func (g *Group) addSession(sess *Session) {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.conns[sess.CID()] = sess
	if uid := sess.UID(); uid > 0 {
		g.users[uid] = sess
	}
}

// 移除会话
func (g *Group) remSession(cid int64) error {
	g.rw.Lock()
	defer g.rw.Unlock()

	sess, ok := g.conns[cid]
	if !ok {
		return ErrNotFoundSession
	}

	delete(g.conns, cid)

	if uid := sess.UID(); uid > 0 {
		delete(g.users, uid)
	}

	return nil
}

// 添加用户会话
func (g *Group) addUserSession(uid int64, sess *Session) {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.users[uid] = sess
}

// 移除用户会话
func (g *Group) remUserSession(uid int64) {
	g.rw.Lock()
	defer g.rw.Unlock()

	delete(g.users, uid)
}
