/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/3/29 3:59 下午
 * @Desc: TODO
 */

package network

import (
	"errors"
	"net"
)

const (
	ConnOpened ConnState = iota + 1 // 连接打开
	ConnHanged                      // 连接挂起
	ConnClosed                      // 连接关闭
)

var (
	ErrConnectionHanged  = errors.New("connection is hanged")
	ErrConnectionClosed  = errors.New("connection is closed")
	ErrIllegalMsgType    = errors.New("illegal message type")
	ErrTooManyConnection = errors.New("too many connection")
)

type (
	ConnState int32

	Conn interface {
		// ID 获取连接ID
		ID() int64
		// UID 获取用户ID
		UID() int64
		// Bind 绑定用户ID
		Bind(uid int64)
		// Unbind 解绑用户ID
		Unbind(uid int64)
		// Send 发送消息（同步）
		Send(msg []byte, msgType ...int) error
		// Push 发送消息（异步）
		Push(msg []byte, msgType ...int) error
		// State 获取连接状态
		State() ConnState
		// Close 关闭连接
		Close(isForce ...bool) error
		// LocalIP 获取本地IP
		LocalIP() (string, error)
		// LocalAddr 获取本地地址
		LocalAddr() (net.Addr, error)
		// RemoteIP 获取远端IP
		RemoteIP() (string, error)
		// RemoteAddr 获取远端地址
		RemoteAddr() (net.Addr, error)
	}
)
