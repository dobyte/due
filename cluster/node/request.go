/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/19 12:20 下午
 * @Desc: TODO
 */

package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/task"
	"github.com/jinzhu/copier"
	"sync"
	"sync/atomic"
)

type request struct {
	node    *Node
	ctx     context.Context  // 上下文
	gid     string           // 来源网关ID
	nid     string           // 来源节点ID
	cid     int64            // 连接ID
	uid     int64            // 用户ID
	message *cluster.Message // 请求消息
	pool    *sync.Pool       // 对象池
	version atomic.Int32     // 对象版本号
}

// GID 获取网关ID
func (r *request) GID() string {
	return r.gid
}

// NID 获取节点ID
func (r *request) NID() string {
	return r.nid
}

// CID 获取连接ID
func (r *request) CID() int64 {
	return r.cid
}

// UID 获取用户ID
func (r *request) UID() int64 {
	return r.uid
}

// Seq 获取消息序列号
func (r *request) Seq() int32 {
	return r.message.Seq
}

// Route 获取消息路由号
func (r *request) Route() int32 {
	return r.message.Route
}

// Event 获取事件类型
func (r *request) Event() cluster.Event {
	return 0
}

// Parse 解析消息
func (r *request) Parse(v interface{}) error {
	msg, ok := r.message.Data.([]byte)
	if !ok {
		return copier.CopyWithOption(v, r.message.Data, copier.Option{
			DeepCopy: true,
		})
	}

	if len(msg) == 0 {
		return nil
	}

	if r.gid != "" && r.node.opts.encryptor != nil {
		data, err := r.node.opts.encryptor.Decrypt(msg)
		if err != nil {
			return err
		}

		return r.node.opts.codec.Unmarshal(data, v)
	}

	return r.node.opts.codec.Unmarshal(msg, v)
}

// Clone 克隆Context
func (r *request) Clone() Context {
	return &request{
		node: r.node,
		pool: r.pool,
		gid:  r.gid,
		nid:  r.nid,
		cid:  r.cid,
		uid:  r.uid,
		ctx:  context.Background(),
		message: &cluster.Message{
			Seq:   r.message.Seq,
			Route: r.message.Route,
			Data:  r.message.Data,
		},
	}
}

// Task 投递任务
func (r *request) Task(fn func(ctx Context)) {
	version := r.version.Add(1)

	task.AddTask(func() {
		defer func() {
			if r.version.CompareAndSwap(version, 0) {
				r.recycle()
			}
		}()

		fn(r)
	})
}

// Proxy 获取代理API
func (r *request) Proxy() *Proxy {
	return r.node.proxy
}

// Context 获取上下文
func (r *request) Context() context.Context {
	return r.ctx
}

// BindGate 绑定网关
func (r *request) BindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return r.node.proxy.BindGate(r.ctx, r.gid, r.cid, uid[0])
	case r.uid != 0:
		return r.node.proxy.BindGate(r.ctx, r.gid, r.cid, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindGate 解绑网关
func (r *request) UnbindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return r.node.proxy.UnbindGate(r.ctx, uid[0])
	case r.uid != 0:
		return r.node.proxy.UnbindGate(r.ctx, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// BindNode 绑定节点
func (r *request) BindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return r.node.proxy.BindNode(r.ctx, uid[0])
	case r.uid != 0:
		return r.node.proxy.BindNode(r.ctx, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindNode 解绑节点
func (r *request) UnbindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return r.node.proxy.UnbindNode(r.ctx, uid[0])
	case r.uid != 0:
		return r.node.proxy.UnbindNode(r.ctx, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// GetIP 获取客户端IP
func (r *request) GetIP() (string, error) {
	if r.gid == "" {
		return "", errors.ErrIllegalOperation
	}

	return r.node.proxy.GetIP(r.ctx, &cluster.GetIPArgs{
		GID:    r.gid,
		Kind:   session.Conn,
		Target: r.cid,
	})
}

// Reply 回复消息
func (r *request) Reply(message *cluster.Message) error {
	switch {
	case r.gid != "":
		return r.node.proxy.Push(r.ctx, &cluster.PushArgs{
			GID:     r.gid,
			Kind:    session.Conn,
			Target:  r.cid,
			Message: message,
		})
	case r.nid != "":
		return r.node.proxy.Deliver(r.ctx, &cluster.DeliverArgs{
			NID:     r.nid,
			UID:     r.uid,
			Message: message,
		})
	default:
		return errors.ErrIllegalOperation
	}
}

// Response 响应消息
func (r *request) Response(message interface{}) error {
	return r.Reply(&cluster.Message{
		Route: r.message.Route,
		Seq:   r.message.Seq,
		Data:  message,
	})
}

// Disconnect 关闭来自网关的连接
func (r *request) Disconnect(force ...bool) error {
	if r.gid == "" {
		return errors.ErrIllegalOperation
	}

	return r.node.proxy.Disconnect(r.ctx, &cluster.DisconnectArgs{
		GID:    r.gid,
		Kind:   session.Conn,
		Target: r.cid,
		Force:  len(force) > 0 && force[0],
	})
}

// 回收请求
func (r *request) recycle() {
	r.pool.Put(r)
}
