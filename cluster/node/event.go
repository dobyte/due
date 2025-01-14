package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/chains"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/transport"
	"sync/atomic"
	"time"
)

type event struct {
	node    *Node           // 代理API
	ctx     context.Context // 上下文
	gid     string          // 网关ID
	cid     int64           // 连接ID
	uid     int64           // 用户ID
	event   cluster.Event   // 时间类型
	version atomic.Int32    // 对象版本号
	chain   *chains.Chain   // defer 调用链
	actor   atomic.Value    // 当前Actor
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
	return e.event
}

// Kind 上下文消息类型
func (e *event) Kind() Kind {
	return Event
}

// Parse 解析消息
func (e *event) Parse(v interface{}) error {
	return errors.NewError(errors.ErrIllegalOperation)
}

// Defer 添加defer延迟调用栈
// 此方法功能与go defer一致，作用域也仅限于当前handler处理函数内，推荐使用Defer方法替代go defer使用
// 区别在于使用Defer方法可以对调用栈进行取消操作
// 同时，在调用Task和Next方法是会自动取消调用栈
// 也可通过Cancel方法进行手动取消
// bottom用于标识是否挂载到栈底部
func (e *event) Defer(fn func(), bottom ...bool) {
	if e.chain == nil {
		e.chain = chains.NewChain()
	}

	if len(bottom) > 0 && bottom[0] {
		e.chain.AddToTail(fn)
	} else {
		e.chain.AddToHead(fn)
	}
}

// Cancel 取消Defer调用栈
func (e *event) Cancel() {
	if e.chain != nil {
		e.chain.Cancel()
	}
}

// 执行defer调用栈
func (e *event) compareVersionExecDefer(version int32) {
	if e.chain != nil && e.version.Load() == version {
		e.chain.FireHead()
	}
}

// Clone 克隆Context
func (e *event) Clone() Context {
	return &event{
		node: e.node,
		gid:  e.gid,
		cid:  e.cid,
		uid:  e.uid,
		ctx:  context.Background(),
	}
}

// Task 投递任务
// 调用此方法会自动取消Defer调用栈的所有执行函数
func (e *event) Task(fn func(ctx Context)) {
	version := e.incrVersion()

	e.Cancel()

	e.node.addWait()

	task.AddTask(func() {
		defer e.compareVersionRecycle(version)

		defer e.compareVersionExecDefer(version)

		fn(e)

		e.node.doneWait()
	})
}

// Next 消息下放
// 调用此方法会自动取消Defer调用栈的所有执行函数
func (e *event) Next() error {
	return e.node.scheduler.dispatch(e)
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

// BindActor 绑定Actor
func (e *event) BindActor(kind, id string) error {
	return e.node.scheduler.bindActor(e.uid, kind, id)
}

// UnbindActor 解绑Actor
func (e *event) UnbindActor(kind string) {
	e.node.scheduler.unbindActor(e.uid, kind)
}

// Spawn 衍生出一个新的Actor
func (e *event) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	return e.node.scheduler.spawn(creator, opts...)
}

// Kill 杀死存在的一个Actor
func (e *event) Kill(kind, id string) bool {
	return e.node.scheduler.kill(kind, id)
}

// Actor 获取Actor
func (e *event) Actor(kind, id string) (*Actor, bool) {
	return e.node.scheduler.load(kind, id)
}

// Invoke 调用函数（线程安全）
// ctx在全局的处理器中，调用的就是proxy.Invoke
// ctx在Actor的处理器中，调用的就是actor.Invoke
func (e *event) Invoke(fn func()) {
	if actor := e.actor.Load().(*Actor); actor != nil {
		actor.Invoke(fn)
	} else {
		e.node.proxy.Invoke(fn)
	}
}

// AfterFunc 延迟调用，与官方的time.AfterFunc用法一致
// ctx在全局的处理器中，调用的就是proxy.AfterFunc
// ctx在Actor的处理器中，调用的就是actor.AfterFunc
func (e *event) AfterFunc(d time.Duration, f func()) *Timer {
	if actor := e.actor.Load().(*Actor); actor != nil {
		return actor.AfterFunc(d, f)
	} else {
		return e.node.proxy.AfterFunc(d, f)
	}
}

// AfterInvoke 延迟调用（线程安全）
// ctx在全局的处理器中，调用的就是proxy.AfterInvoke
// ctx在Actor的处理器中，调用的就是actor.AfterInvoke
func (e *event) AfterInvoke(d time.Duration, f func()) *Timer {
	if actor := e.actor.Load().(*Actor); actor != nil {
		return actor.AfterInvoke(d, f)
	} else {
		return e.node.proxy.AfterInvoke(d, f)
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

// Deliver 投递消息给节点处理
func (e *event) Deliver(args *cluster.DeliverArgs) error {
	return e.node.proxy.Deliver(e.ctx, args)
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

// NewMeshClient 新建微服务客户端
// target参数可分为三种种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
func (e *event) NewMeshClient(target string) (transport.Client, error) {
	return e.node.proxy.NewMeshClient(target)
}

// 保存当前Actor
func (e *event) storeActor(actor *Actor) {
	e.actor.Store(actor)
}

// 增长版本号
func (e *event) incrVersion() int32 {
	return e.version.Add(1)
}

// 获取版本号
func (e *event) loadVersion() int32 {
	return e.version.Load()
}

// 比对版本号后进行回收对象
func (e *event) compareVersionRecycle(version int32) {
	if e.version.CompareAndSwap(version, 0) {
		if e.chain != nil {
			e.chain.Cancel()
			e.chain = nil
		}

		e.actor.Store((*Actor)(nil))

		e.node.evtPool.Put(e)
	}
}
