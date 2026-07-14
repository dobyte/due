package node

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

type Creator func(actor *Actor, args ...any) Processor

const (
	unstart   int32 = iota // 未启动
	started                // 已启动
	destroyed              // 已销毁
)

type Actor struct {
	opts                *actorOptions                  // 配置项
	scheduler           *Scheduler                     // 调度器
	state               atomic.Int32                   // 状态
	routes              map[int32]RouteHandler         // 路由处理器
	events              map[cluster.Event]EventHandler // 事件处理器
	defaultRouteHandler RouteHandler                   // 默认路由处理器
	processor           Processor                      // 处理器
	rw                  sync.RWMutex                   // 锁
	taskQueue           *queue.Queue[func()]           // 任务队列
	messageQueue        *queue.Queue[Context]          // 消息队列
	binds               sync.Map                       // 绑定的用户
}

// ID 获取Actor的ID
func (a *Actor) ID() string {
	return a.opts.id
}

// PID 获取Actor的唯一识别ID
func (a *Actor) PID() string {
	return a.Kind() + "/" + a.ID()
}

// Kind 获取Actor类型
func (a *Actor) Kind() string {
	return a.opts.kind
}

// Spawn 衍生出一个Actor
func (a *Actor) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	return a.scheduler.spawn(creator, opts...)
}

// Proxy 获取代理API
func (a *Actor) Proxy() *Proxy {
	return a.scheduler.node.proxy
}

// Invoke 调用函数（Actor内线程安全）
func (a *Actor) Invoke(f func(), isBlock ...bool) error {
	if a.state.Load() != started {
		return errors.ErrActorNotStarted
	}

	a.rw.RLock()
	err := a.taskQueue.Write(f)
	a.rw.RUnlock()

	return err
}

// AfterFunc 延迟调用，与官方的time.AfterFunc用法一致
func (a *Actor) AfterFunc(d time.Duration, f func()) (*Timer, error) {
	if a.state.Load() != started {
		return nil, errors.ErrActorNotStarted
	}

	timer := time.AfterFunc(d, func() {
		a.rw.RLock()
		defer a.rw.RUnlock()

		if a.state.Load() != started {
			log.Warnf("actor %s write func task failed, err: %v", a.PID(), errors.ErrActorNotStarted)
			return
		}

		xcall.Call(f)
	})

	return &Timer{timer: timer}, nil
}

// AfterInvoke 延迟调用（线程安全）
func (a *Actor) AfterInvoke(d time.Duration, f func()) (*Timer, error) {
	if a.state.Load() != started {
		return nil, errors.ErrActorNotStarted
	}

	timer := time.AfterFunc(d, func() {
		if a.state.Load() != started {
			log.Warnf("actor %s write func task failed, err: %v", a.PID(), errors.ErrActorNotStarted)
			return
		}

		a.rw.RLock()
		err := a.taskQueue.Write(f)
		a.rw.RUnlock()
		if err != nil {
			log.Warnf("actor %s write func task failed, err: %v", a.PID(), err)
		}
	})

	return &Timer{timer: timer}, nil
}

// SetDefaultRouteHandler 设置默认路由处理器
func (a *Actor) SetDefaultRouteHandler(handler RouteHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.defaultRouteHandler = handler
	case started:
		a.taskQueue.Write(func() {
			a.defaultRouteHandler = handler
		})
	default:
		// ignore
	}
}

// AddRouteHandler 添加路由处理器
func (a *Actor) AddRouteHandler(route int32, handler RouteHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.routes[route] = handler
	case started:
		a.taskQueue.Write(func() {
			a.routes[route] = handler

			if a.opts.dispatch {
				a.scheduler.routes.Store(route, a.Kind())
			}
		})
	default:
		// ignore
	}
}

// AddEventHandler 添加事件处理器
func (a *Actor) AddEventHandler(event cluster.Event, handler EventHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.events[event] = handler
	case started:
		a.taskQueue.Write(func() {
			a.events[event] = handler
		})
	default:
		// ignore
	}
}

// Next 投递消息到Actor中进行处理
func (a *Actor) Next(ctx Context) error {
	a.rw.RLock()
	defer a.rw.RUnlock()

	if a.state.Load() != started {
		return errors.ErrActorNotStarted
	}

	ctx.storeActor(a)
	ctx.incrVersion()
	ctx.Cancel()

	return a.messageQueue.Write(ctx)
}

// Deliver 投递消息到当前Actor中进行处理
func (a *Actor) Deliver(uid int64, message *cluster.Message) error {
	buf, err := a.scheduler.node.proxy.PackBuffer(message.Data)
	if err != nil {
		return err
	}

	req := a.scheduler.node.reqPool.Get().(*request)
	req.nid = a.scheduler.node.opts.id
	req.uid = uid
	req.ctx = context.Background()
	req.message.Seq = message.Seq
	req.message.Route = message.Route
	req.message.Data = buf

	return a.Next(req)
}

// Push 推送消息到本地Node队列上进行处理
func (a *Actor) Push(uid int64, message *cluster.Message) error {
	buf, err := a.scheduler.node.proxy.PackBuffer(message.Data)
	if err != nil {
		return err
	}

	return a.scheduler.node.router.deliver("", a.scheduler.node.opts.id, a.PID(), 0, uid, message.Seq, message.Route, buf)
}

// Destroy 销毁Actor
func (a *Actor) Destroy() (ok bool) {
	if ok = a.destroy(); !ok {
		return
	}

	_, ok = a.scheduler.remove(a.Kind(), a.ID())
	return
}

// 销毁Actor
func (a *Actor) destroy() bool {
	if !a.state.CompareAndSwap(started, destroyed) {
		return false
	}

	a.processor.Destroy()

	a.scheduler.batchUnbindActor(func(relations map[int64]map[string]*Actor) {
		a.binds.Range(func(uid, _ any) bool {
			delete(relations[uid.(int64)], a.Kind())
			return true
		})
	})

	a.rw.Lock()
	a.clear()
	a.rw.Unlock()

	return true
}

// 清空Actor资源
func (a *Actor) clear() {
	a.taskQueue.Close()
	a.messageQueue.Close()
	clear(a.routes)
	clear(a.events)
	a.processor = nil
	a.defaultRouteHandler = nil
}

// 绑定用户
func (a *Actor) bindUser(uid int64) {
	a.binds.Store(uid, struct{}{})
}

// 解绑用户
func (a *Actor) unbindUser(uid int64) bool {
	_, ok := a.binds.LoadAndDelete(uid)
	return ok
}

// 分发
func (a *Actor) dispatch() {
	for {
		select {
		case ctx, ok := <-a.messageQueue.Read():
			if !ok {
				return
			}

			version := ctx.loadVersion()

			if ctx.Kind() == Event {
				if handler, ok := a.events[ctx.Event()]; ok {
					xcall.Call(func() { handler(ctx) })

					ctx.compareVersionExecDefer(version)
				}
			} else {
				if handler, ok := a.routes[ctx.Route()]; ok {
					xcall.Call(func() { handler(ctx) })

					ctx.compareVersionExecDefer(version)
				} else if a.defaultRouteHandler != nil {
					xcall.Call(func() { a.defaultRouteHandler(ctx) })

					ctx.compareVersionExecDefer(version)
				}
			}

			ctx.compareVersionRecycle(version)
		case handle, ok := <-a.taskQueue.Read():
			if !ok {
				return
			}

			xcall.Call(handle)
		}
	}
}
