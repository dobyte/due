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
	"github.com/petermattis/goid"
)

// Creator Actor处理器创建函数
type Creator func(actor *Actor, args ...any) Processor

const (
	unstart   int32 = iota // 未启动
	started                // 已启动
	destroyed              // 已销毁
)

// Actor Actor模型
// 拥有独立的消息队列与任务队列，保证其内部处理是线程安全的
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
	dispatchGoid        atomic.Int64                   // 分发器协程ID
}

// ID 获取Actor的ID
// @return @1 string Actor的ID
func (a *Actor) ID() string {
	return a.opts.id
}

// PID 获取Actor的唯一识别ID
// 由Kind与ID组合而成，用于全局唯一定位该Actor
// @return @1 string Actor的唯一识别ID
func (a *Actor) PID() string {
	return a.Kind() + "/" + a.ID()
}

// Kind 获取Actor类型
// @return @1 string Actor类型
func (a *Actor) Kind() string {
	return a.opts.kind
}

// Spawn 衍生出一个Actor
// @param creator Creator Actor处理器创建函数
// @param opts ...ActorOption Actor配置项
// @return @1 *Actor 衍生出的Actor实例
// @return @2 error Actor已存在或创建失败时返回的错误
func (a *Actor) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	return a.scheduler.spawn(creator, opts...)
}

// Proxy 获取代理API
// @return @1 *Proxy 节点代理
func (a *Actor) Proxy() *Proxy {
	return a.scheduler.node.proxy
}

// Invoke 调用函数（Actor内线程安全）
// 任务写入Actor的任务队列串行执行；阻塞模式下会等待函数执行完成
// @param f func() 待调用的函数
// @param wait ...bool 是否等待调用完成，默认不等待
// @return @1 error Actor未启动或任务入队失败时返回的错误
func (a *Actor) Invoke(f func(), wait ...bool) error {
	if !a.started() {
		return errors.ErrActorNotStarted
	}

	if len(wait) > 0 && wait[0] {
		if a.dispatchGoid.Load() == goid.Get() {
			xcall.Call(f)
		} else {
			wg := sync.WaitGroup{}
			wg.Add(1)

			a.rw.RLock()
			err := a.taskQueue.Write(func() {
				defer wg.Done()

				if a.started() {
					f()
				}
			})
			a.rw.RUnlock()

			if err != nil {
				return err
			}

			wg.Wait()
		}
	} else {
		a.rw.RLock()
		err := a.taskQueue.Write(func() {
			if a.started() {
				f()
			}
		})
		a.rw.RUnlock()

		if err != nil {
			return err
		}
	}

	return nil
}

// AfterFunc 延迟调用，与官方的time.AfterFunc用法一致
// @param d time.Duration 延迟时长
// @param f func() 待调用的函数
// @return @1 *Timer 定时器，可通过Stop取消
// @return @2 error Actor未启动时返回的错误
func (a *Actor) AfterFunc(d time.Duration, f func()) (*Timer, error) {
	if !a.started() {
		return nil, errors.ErrActorNotStarted
	}

	timer := time.AfterFunc(d, func() {
		if a.started() {
			xcall.Call(f)
		} else {
			log.Warnf("actor %s exec task failed, err: %v", a.PID(), errors.ErrActorNotStarted)
		}
	})

	return &Timer{timer: timer}, nil
}

// AfterInvoke 延迟调用（线程安全）
// 延迟后通过任务队列串行执行函数，保证Actor内线程安全
// @param d time.Duration 延迟时长
// @param f func() 待调用的函数
// @return @1 *Timer 定时器，可通过Stop取消
// @return @2 error Actor未启动时返回的错误
func (a *Actor) AfterInvoke(d time.Duration, f func()) (*Timer, error) {
	if !a.started() {
		return nil, errors.ErrActorNotStarted
	}

	timer := time.AfterFunc(d, func() {
		var err error

		a.rw.RLock()
		if a.started() {
			err = a.taskQueue.Write(func() {
				if a.started() {
					f()
				}
			})
		} else {
			err = errors.ErrActorNotStarted
		}
		a.rw.RUnlock()

		if err != nil {
			log.Warnf("actor %s exec task failed, err: %v", a.PID(), err)
		}
	})

	return &Timer{timer: timer}, nil
}

// SetDefaultRouteHandler 设置默认路由处理器
// @param handler RouteHandler 默认路由处理器，所有未注册路由均由其处理
func (a *Actor) SetDefaultRouteHandler(handler RouteHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.defaultRouteHandler = handler
	case started:
		a.taskQueue.Write(func() {
			if a.started() {
				a.defaultRouteHandler = handler
			}
		})
	default:
		// ignore
	}
}

// AddRouteHandler 添加路由处理器
// @param route int32 路由号
// @param handler RouteHandler 路由处理函数
func (a *Actor) AddRouteHandler(route int32, handler RouteHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.routes[route] = handler
	case started:
		a.taskQueue.Write(func() {
			if a.started() {
				a.routes[route] = handler

				if a.opts.dispatch {
					a.scheduler.routes.Store(route, a.Kind())
				}
			}
		})
	default:
		// ignore
	}
}

// AddEventHandler 添加事件处理器
// @param event cluster.Event 事件类型
// @param handler EventHandler 事件处理函数
func (a *Actor) AddEventHandler(event cluster.Event, handler EventHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.events[event] = handler
	case started:
		a.taskQueue.Write(func() {
			if a.started() {
				a.events[event] = handler
			}
		})
	default:
		// ignore
	}
}

// Next 投递消息到Actor中进行处理
// @param ctx Context 消息上下文，写入Actor消息队列串行处理
// @return @1 error Actor未启动或消息入队失败时返回的错误
func (a *Actor) Next(ctx Context) error {
	a.rw.RLock()
	defer a.rw.RUnlock()

	if a.state.Load() != started {
		return errors.ErrActorNotStarted
	}

	ctx.storeActor(a)
	ctx.incrVersion()
	ctx.cancelDefer()

	if err := a.messageQueue.Write(ctx); err != nil {
		ctx.deleteActor()
		ctx.decrVersion()
		ctx.recoverDefer()
		return err
	}

	return nil
}

// Deliver 投递消息到当前Actor中进行处理
// @param uid int64 用户ID
// @param message *cluster.Message 待投递的消息
// @return @1 error 打包消息或投递失败时返回的错误
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
// @param uid int64 用户ID
// @param message *cluster.Message 待推送的消息
// @return @1 error 打包消息或推送失败时返回的错误
func (a *Actor) Push(uid int64, message *cluster.Message) error {
	buf, err := a.scheduler.node.proxy.PackBuffer(message.Data)
	if err != nil {
		return err
	}

	return a.scheduler.node.router.deliver("", a.scheduler.node.opts.id, a.PID(), 0, uid, message.Seq, message.Route, buf)
}

// Destroy 销毁Actor
// @return @1 bool 是否成功销毁，Actor不存在或已销毁时返回false
func (a *Actor) Destroy() (ok bool) {
	if ok = a.destroy(); !ok {
		return
	}

	_, ok = a.scheduler.remove(a.Kind(), a.ID())
	return
}

// 销毁Actor
// 批量解绑用户、关闭任务与消息队列，释放队列中的残留消息并回调处理器销毁方法
// @return @1 bool 是否成功销毁，Actor非启动状态时返回false
func (a *Actor) destroy() bool {
	if !a.state.CompareAndSwap(started, destroyed) {
		return false
	}

	a.scheduler.batchUnbindActor(func(relations map[int64]map[string]*Actor) {
		a.binds.Range(func(uid, _ any) bool {
			delete(relations[uid.(int64)], a.Kind())
			return true
		})
	})

	a.rw.Lock()
	processor := a.processor
	a.processor = nil
	a.taskQueue.Close()
	a.messageQueue.Close()
	a.rw.Unlock()

	// 释放掉所有任务队列中的任务
	for handle := range a.taskQueue.Read() {
		xcall.Call(handle)
	}

	// 释放掉所有消息队列中的消息
	for ctx := range a.messageQueue.Read() {
		ctx.release()
	}

	if processor != nil {
		xcall.Call(processor.Destroy)
	}

	if a.opts.wait {
		a.scheduler.node.doDoneWait()
	}

	return true
}

// 绑定用户
// @param uid int64 用户ID
func (a *Actor) bindUser(uid int64) {
	a.binds.Store(uid, struct{}{})
}

// 解绑用户
// @param uid int64 待解绑的用户ID
// @return @1 bool 用户是否已绑定到当前Actor
func (a *Actor) unbindUser(uid int64) bool {
	_, ok := a.binds.LoadAndDelete(uid)
	return ok
}

// 分发
// 循环监听消息队列与任务队列并分别处理事件/路由消息与任务，队列关闭时退出
func (a *Actor) dispatch() {
	a.dispatchGoid.Store(goid.Get())

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

// 是否已启动
// @return @1 bool 是否处于启动状态
func (a *Actor) started() bool {
	return a.state.Load() == started
}
