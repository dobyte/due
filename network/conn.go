/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/3/29 3:59 下午
 * @Desc: TODO
 */

package network

import (
	"net"
)

const (
	ConnOpened ConnState = iota + 1 // 连接打开
	ConnHanged                      // 连接挂起
	ConnClosed                      // 连接关闭
)

type (
	// ConnState 连接状态
	ConnState int32

	// Conn 连接接口
	Conn interface {
		// ID 获取连接ID
		// @return @1 int64 连接ID
		ID() int64
		// UID 获取用户ID
		// @return @1 int64 用户ID
		UID() int64
		// Attr 获取属性接口
		// @return @1 Attr 属性接口
		Attr() Attr
		// Bind 绑定用户ID
		// @param uid int64 用户ID
		// @return @1 error 错误信息
		Bind(uid int64) error
		// Unbind 解绑用户ID
		// @return @1 error 错误信息
		Unbind() error
		// Send 高优先级发送消息
		// @param msg []byte 消息内容
		// @return @1 error 错误信息
		Send(msg []byte) error
		// Push 低优先级发送消息
		// @param msg []byte 消息内容
		// @return @1 error 错误信息
		Push(msg []byte) error
		// State 获取连接状态
		// @return @1 ConnState 连接状态
		State() ConnState
		// Close 关闭连接
		// @param force ...bool 是否强制关闭
		// @return @1 error 错误信息
		Close(force ...bool) error
		// LocalIP 获取本地IP
		// @return @1 string 本地IP地址
		// @return @2 error 错误信息
		LocalIP() (string, error)
		// LocalAddr 获取本地地址
		// @return @1 net.Addr 本地地址
		// @return @2 error 错误信息
		LocalAddr() (net.Addr, error)
		// RemoteIP 获取远端IP
		// @return @1 string 远端IP地址
		// @return @2 error 错误信息
		RemoteIP() (string, error)
		// RemoteAddr 获取远端地址
		// @return @1 net.Addr 远端地址
		// @return @2 error 错误信息
		RemoteAddr() (net.Addr, error)
	}

	// Attr 连接属性接口
	Attr interface {
		// Set 设置属性值
		// @param key any 属性键
		// @param value any 属性值
		Set(key, value any)
		// Get 获取属性值
		// @param key any 属性键
		// @return @1 any 属性值
		// @return @2 bool 是否存在
		Get(key any) (any, bool)
		// Del 删除属性值
		// @param key any 属性键
		// @return @1 bool 是否删除成功
		Del(key any) bool
		// Clear 清空所有属性值
		Clear()
		// Visit 访问所有的属性值
		// @param fn func(key, value any) bool 遍历函数，返回 false 时停止遍历
		Visit(fn func(key, value any) bool)
	}
)
