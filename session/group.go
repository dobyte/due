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
	ErrSessionNotFound    = errors.New("the session not found in the group")
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
	return &Group{conns: make(map[int64]*Session), users: make(map[int64]*Session)}
}

// AddSession 添加会话
func (g *Group) AddSession(session *Session) {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.conns[session.CID()] = session
	if uid := session.UID(); uid > 0 {
		g.users[uid] = session
	}
	session.joinGroup(g)
}

// 加入群组
func (g *Group) add(session *Session) {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.conns[session.CID()] = session
	if uid := session.UID(); uid > 0 {
		g.users[uid] = session
	}
}

// 添加映射关系
func (g *Group) addUserMap(uid int64, session *Session) {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.users[uid] = session
}

// RemSession 移除会话
func (g *Group) RemSession(kind Kind, target int64) (*Session, error) {
	g.rw.Lock()
	defer g.rw.Unlock()

	switch kind {
	case Conn:
		session, ok := g.conns[target]
		if !ok {
			return nil, ErrSessionNotFound
		}

		if uid := session.UID(); uid > 0 {
			delete(g.users, uid)
		}
		delete(g.conns, target)
		session.quitGroup(g)
		return session, nil
	case User:
		session, ok := g.users[target]
		if !ok {
			return nil, ErrSessionNotFound
		}

		delete(g.users, target)
		delete(g.conns, session.CID())
		session.quitGroup(g)
		return session, nil
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
	var sessions map[int64]*Session
	switch kind {
	case Conn:
		sessions = g.conns
	case User:
		sessions = g.users
	default:
		return nil, ErrInvalidSessionKind
	}

	session, ok := sessions[target]
	if !ok {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// 移除会话
func (g *Group) remSession(cid int64) error {
	g.rw.Lock()
	defer g.rw.Unlock()

	session, ok := g.conns[cid]
	if !ok {
		return ErrSessionNotFound
	}

	delete(g.conns, cid)
	if uid := session.UID(); uid > 0 {
		delete(g.users, uid)
	}

	return nil
}

// Send 发送消息（同步）
func (g *Group) Send(kind Kind, target int64, msg []byte, typ ...int) error {
	g.rw.RLock()
	defer g.rw.RUnlock()

	session, err := g.getSession(kind, target)
	if err != nil {
		return err
	}
	return session.Send(msg, typ...)
}

// Push 推送消息（异步）
func (g *Group) Push(kind Kind, target int64, msg []byte, msgType ...int) error {
	g.rw.RLock()
	defer g.rw.RUnlock()

	session, err := g.getSession(kind, target)
	if err != nil {
		return err
	}
	return session.Push(msg, msgType...)
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
