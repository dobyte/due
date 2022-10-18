package transport

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/session"
)

type Server interface {
	// Addr 监听地址
	Addr() string
	// Scheme 协议
	Scheme() string
	// Endpoint 服务端口
	Endpoint() *endpoint.Endpoint
	// Start 启动服务器
	Start() error
	// Stop 停止服务器
	Stop() error
}

type GateProvider interface {
	// Session 获取会话
	Session(kind session.Kind, target int64) (*session.Session, error)
	// Bind 绑定用户与网关间的关系
	Bind(ctx context.Context, uid int64) error
	// Unbind 解绑用户与网关间的关系
	Unbind(ctx context.Context, uid int64) error
	// Push 发送消息（异步）
	Push(kind session.Kind, target int64, msg []byte, msgType ...int) error
	// Multicast 推送组播消息（异步）
	Multicast(kind session.Kind, targets []int64, msg []byte, msgType ...int) (n int, err error)
	// Broadcast 推送广播消息（异步）
	Broadcast(kind session.Kind, msg []byte, msgType ...int) (n int, err error)
}

type NodeProvider interface {
	// LocateNode 定位用户所在节点
	LocateNode(ctx context.Context, uid int64) (nid string, miss bool, err error)
	// CheckRouteStateful 检测某个路由是否为有状态路由
	CheckRouteStateful(route int32) (stateful bool, exist bool)
	// Trigger 触发事件
	Trigger(event cluster.Event, gid string, uid int64)
	// Deliver 投递消息
	Deliver(gid, nid string, cid, uid int64, message *Message)
}
