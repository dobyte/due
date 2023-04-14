/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/9 20:10
 * @Desc: TODO
 */

package session

import (
	"net"
	"sync"

	"github.com/dobyte/due/network"
)

type Session struct {
	rw     sync.RWMutex        // 读写锁
	conn   network.Conn        // 连接
	groups map[*Group]struct{} // 所在组
}

func NewSession() *Session {
	return &Session{}
}

// Init 初始化会话
func (s *Session) Init(conn network.Conn) {
	s.rw.Lock()
	defer s.rw.Unlock()

	s.conn = conn
	s.groups = make(map[*Group]struct{})
}

// Reset 重置会话
func (s *Session) Reset() {
	s.rw.Lock()
	defer s.rw.Unlock()

	s.groups = nil
}

// CID 获取连接ID
func (s *Session) CID() int64 {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.ID()
}

// UID 获取用户ID
func (s *Session) UID() int64 {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.UID()
}

// Bind 绑定用户ID
func (s *Session) Bind(uid int64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	s.conn.Bind(uid)
	for group := range s.groups {
		group.addUserSession(uid, s)
	}
}

// Unbind 解绑用户ID
func (s *Session) Unbind(uid int64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	s.conn.Unbind()
	for group := range s.groups {
		group.remUserSession(uid)
	}
}

// Close 关闭会话
func (s *Session) Close(isForce ...bool) error {
	s.rw.Lock()
	defer s.rw.Unlock()

	return s.conn.Close(isForce...)
}

// LocalIP 获取本地IP
func (s *Session) LocalIP() (string, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.LocalIP()
}

// LocalAddr 获取本地地址
func (s *Session) LocalAddr() (net.Addr, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.LocalAddr()
}

// RemoteIP 获取远端IP
func (s *Session) RemoteIP() (string, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.RemoteIP()
}

// RemoteAddr 获取远端地址
func (s *Session) RemoteAddr() (net.Addr, error) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.RemoteAddr()
}

// Send 发送消息（同步）
func (s *Session) Send(msg []byte, msgType ...int) error {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.Send(msg, msgType...)
}

// Push 发送消息（异步）
func (s *Session) Push(msg []byte, msgType ...int) error {
	s.rw.RLock()
	defer s.rw.RUnlock()

	return s.conn.Push(msg, msgType...)
}

// AddToGroups 添加到会话组
func (s *Session) AddToGroups(groups ...*Group) {
	s.rw.Lock()
	defer s.rw.Unlock()

	for i := range groups {
		group := groups[i]
		group.addSession(s)
		s.groups[group] = struct{}{}
	}
}

// 添加到会话组
func (s *Session) addToGroups(groups ...*Group) {
	s.rw.Lock()
	defer s.rw.Unlock()

	for i := range groups {
		s.groups[groups[i]] = struct{}{}
	}
}

// RemFromGroups 从会话组移除
func (s *Session) RemFromGroups(groups ...*Group) {
	s.rw.Lock()
	defer s.rw.Unlock()

	for _, group := range groups {
		_ = group.remSession(s.CID())
		delete(s.groups, group)
	}
}

// 从会话组移除
func (s *Session) remFromGroups(groups ...*Group) {
	s.rw.Lock()
	defer s.rw.Unlock()

	for _, group := range groups {
		delete(s.groups, group)
	}
}
