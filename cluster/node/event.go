package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/task"
	"sync"
	"sync/atomic"
)

type event struct {
	node    *Node           // 代理API
	ctx     context.Context // 上下文
	gid     string          // 网关ID
	cid     int64           // 连接ID
	uid     int64           // 用户ID
	kind    cluster.Event   // 时间类型
	pool    *sync.Pool      // 对象池
	version atomic.Int32    // 对象版本号
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
		node: e.node,
		pool: e.pool,
		gid:  e.gid,
		cid:  e.cid,
		uid:  e.uid,
		ctx:  context.Background(),
	}
}

// Task 投递任务
func (e *event) Task(fn func(ctx Context)) {
	version := e.version.Add(1)

	task.AddTask(func() {
		defer func() {
			if e.version.CompareAndSwap(version, 0) {
				e.recycle()
			}
		}()

		fn(e)
	})
}

// Proxy 获取代理API
func (e *event) Proxy() *Proxy {
	return e.node.proxy
}

// Context 获取上下文
func (e *event) Context() context.Context {
	return e.ctx
}

// BindGate 绑定网关
func (e *event) BindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.node.proxy.BindGate(e.ctx, e.gid, e.cid, uid[0])
	case e.uid != 0:
		return e.node.proxy.BindGate(e.ctx, e.gid, e.cid, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindGate 解绑网关
func (e *event) UnbindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.node.proxy.UnbindGate(e.ctx, uid[0])
	case e.uid != 0:
		return e.node.proxy.UnbindGate(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// BindNode 绑定节点
func (e *event) BindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.node.proxy.BindNode(e.ctx, uid[0])
	case e.uid != 0:
		return e.node.proxy.BindNode(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindNode 解绑节点
func (e *event) UnbindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.node.proxy.UnbindNode(e.ctx, uid[0])
	case e.uid != 0:
		return e.node.proxy.UnbindNode(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// GetIP 获取客户端IP
func (e *event) GetIP() (string, error) {
	return e.node.proxy.GetIP(e.ctx, &cluster.GetIPArgs{
		GID:    e.gid,
		Kind:   session.Conn,
		Target: e.cid,
	})
}

// Reply 回复消息
func (e *event) Reply(message *cluster.Message) error {
	return e.node.proxy.Push(e.ctx, &cluster.PushArgs{
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
func (e *event) Disconnect(force ...bool) error {
	return e.node.proxy.Disconnect(e.ctx, &cluster.DisconnectArgs{
		GID:    e.gid,
		Kind:   session.Conn,
		Target: e.cid,
		Force:  len(force) > 0 && force[0],
	})
}

// 回收事件
func (e *event) recycle() {
	e.pool.Put(e)
}
