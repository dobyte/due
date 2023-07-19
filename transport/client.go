package transport

import (
	"context"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
)

type NodeClient interface {
	// Trigger 触发事件
	Trigger(ctx context.Context, args *TriggerArgs) (miss bool, err error)
	// Deliver 投递消息
	Deliver(ctx context.Context, args *DeliverArgs) (miss bool, err error)
}

type GateClient interface {
	// Bind 绑定用户与连接
	Bind(ctx context.Context, cid, uid int64) (miss bool, err error)
	// Unbind 解绑用户与连接
	Unbind(ctx context.Context, uid int64) (miss bool, err error)
	// GetIP 获取客户端IP
	GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, miss bool, err error)
	// Push 推送消息
	Push(ctx context.Context, kind session.Kind, target int64, message *packet.Message) (miss bool, err error)
	// Multicast 推送组播消息
	Multicast(ctx context.Context, kind session.Kind, targets []int64, message *packet.Message) (total int64, err error)
	// Broadcast 推送广播消息
	Broadcast(ctx context.Context, kind session.Kind, message *packet.Message) (total int64, err error)
	// Stat 统计会话总数
	Stat(ctx context.Context, kind session.Kind) (total int64, err error)
	// Disconnect 断开连接
	Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (miss bool, err error)
}

type ServiceClient interface {
	// Call 调用服务方法
	Call(ctx context.Context, service, method string, args interface{}, reply interface{}, opts ...interface{}) error
	// Client 获取内部客户端
	Client() interface{}
}

type NewServiceClientFunc func(target string) (ServiceClient, error)
