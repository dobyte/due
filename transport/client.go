package transport

import (
	"context"
)

type Client interface {
	Call(ctx context.Context)
}

type NodeClient interface {
	// Trigger 触发事件
	Trigger(ctx context.Context, req *TriggerRequest) (*TriggerReply, error)
	// Deliver 投递消息
	Deliver(ctx context.Context, req *DeliverRequest) (*DeliverReply, error)
}

type GateClient interface {
	// Bind 绑定用户与连接
	Bind(ctx context.Context, req *BindRequest) (*BindReply, error)
	// Unbind 解绑用户与连接
	Unbind(ctx context.Context, req *UnbindRequest) (*UnbindReply, error)
}

type TriggerRequest struct {
	Event int32  // 事件
	GID   string // 网关ID
	UID   int64  // 用户ID
}

type TriggerReply struct {
}

type DeliverRequest struct {
	GID     string   // 网关ID
	NID     string   // 节点ID
	CID     int64    // 连接ID
	UID     int64    // 用户ID
	Message *Message // 消息
}

type DeliverReply struct {
}

type Message struct {
	Seq    int32  // 序列号
	Route  int32  // 路由
	Buffer []byte // 消息内容
}

type BindRequest struct {
	CID int64 // 连接ID
	UID int64 // 用户ID
}

type BindReply struct {
}

type UnbindRequest struct {
	UID int64 // 用户ID
}

type UnbindReply struct {
}

type GetIPRequest struct {
	NID    string // 节点ID
	Kind   int32  // 推送类型 1：CID 2：UID
	Target int64  // 推送目标
}

type GetIPReply struct {
	IP string // IP地址
}
