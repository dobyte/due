package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/session"
)

type event struct {
	proxy *Proxy
	ctx   context.Context
	gid   string
	cid   int64
	uid   int64
	kind  cluster.Event
}

// GID 获取网关ID
func (e *event) GID() string {
	return e.gid
}

// NID 获取节点ID
func (e *event) NID() string {
	return ""
}

// CID 获取连接ID
func (e *event) CID() int64 {
	return e.cid
}

// UID 获取用户ID
func (e *event) UID() int64 {
	return e.uid
}

// Seq 获取消息序列号
func (e *event) Seq() int32 {
	return 0
}

// Route 获取消息路由号
func (e *event) Route() int32 {
	return 0
}

// Event 获取事件类型
func (e *event) Event() cluster.Event {
	return e.kind
}

// Parse 解析消息
func (e *event) Parse(v interface{}) error {
	return errors.NewError(errors.ErrIllegalOperation)
}

// Clone 克隆Context
func (e *event) Clone() Context {
	return &event{
		ctx:   context.Background(),
		gid:   e.gid,
		cid:   e.cid,
		uid:   e.uid,
		proxy: e.proxy,
	}
}

// Proxy 获取代理API
func (e *event) Proxy() *Proxy {
	return e.proxy
}

// Context 获取上下文
func (e *event) Context() context.Context {
	return e.ctx
}

// BindGate 绑定网关
func (e *event) BindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.proxy.BindGate(e.ctx, uid[0], e.gid, e.cid)
	case e.uid != 0:
		return e.proxy.BindGate(e.ctx, e.uid, e.gid, e.cid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindGate 解绑网关
func (e *event) UnbindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.proxy.UnbindGate(e.ctx, uid[0])
	case e.uid != 0:
		return e.proxy.UnbindGate(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// BindNode 绑定节点
func (e *event) BindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.proxy.BindNode(e.ctx, uid[0])
	case e.uid != 0:
		return e.proxy.BindNode(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindNode 解绑节点
func (e *event) UnbindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.proxy.UnbindNode(e.ctx, uid[0])
	case e.uid != 0:
		return e.proxy.UnbindNode(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// GetIP 获取客户端IP
func (e *event) GetIP() (string, error) {
	return e.proxy.GetIP(e.ctx, &cluster.GetIPArgs{
		GID:    e.gid,
		Kind:   session.Conn,
		Target: e.cid,
	})
}

// Reply 回复消息
func (e *event) Reply(message *cluster.Message) error {
	return e.proxy.Push(e.ctx, &cluster.PushArgs{
		GID:     e.gid,
		Kind:    session.Conn,
		Target:  e.cid,
		Message: message,
	})
}

// Response 响应消息
func (e *event) Response(message interface{}) error {
	return errors.NewError(errors.ErrIllegalOperation)
}

// Disconnect 关闭来自网关的连接
func (e *event) Disconnect(isForce ...bool) error {
	return e.proxy.Disconnect(e.ctx, &cluster.DisconnectArgs{
		GID:     e.gid,
		Kind:    session.Conn,
		Target:  e.cid,
		IsForce: len(isForce) > 0 && isForce[0],
	})
}
