package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
)

type Context interface {
	// GID 获取网关ID
	GID() string
	// NID 获取节点ID
	NID() string
	// CID 获取连接ID
	CID() int64
	// UID 获取用户ID
	UID() int64
	// Seq 获取消息序列号
	Seq() int32
	// Route 获取消息路由号
	Route() int32
	// Event 获取事件类型
	Event() cluster.Event
	// Parse 解析消息
	Parse(v interface{}) error
	// Clone 克隆Context
	Clone() Context
	// Task 投递任务
	Task(fn func(ctx Context))
	// Proxy 获取代理API
	Proxy() *Proxy
	// Context 获取上下文
	Context() context.Context
	// BindGate 绑定网关
	BindGate(uid ...int64) error
	// UnbindGate 解绑网关
	UnbindGate(uid ...int64) error
	// BindNode 绑定节点
	BindNode(uid ...int64) error
	// UnbindNode 解绑节点
	UnbindNode(uid ...int64) error
	// GetIP 获取客户端IP
	GetIP() (string, error)
	// Reply 回复消息
	Reply(message *cluster.Message) error
	// Response 响应消息
	Response(message interface{}) error
	// Disconnect 关闭来自网关的连接
	Disconnect(isForce ...bool) error
}
