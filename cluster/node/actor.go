package node

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/utils/xcall"
)

type Creator func(actor *Actor, args ...any) Processor

type Actor struct {
	opts      *actorOptions                  // 配置项
	scheduler *Scheduler                     // 调度器
	routes    map[int32]RouteHandler         // 路由处理器
	events    map[cluster.Event]EventHandler // 事件处理器
	processor Processor                      // 处理器
	mailbox   chan Context                   // 邮箱
	fnChan    chan func()
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
	return a.processor.Kind()
}

// Spawn 衍生出一个Actor
func (a *Actor) Spawn(creator Creator, opts ...ActorOption) *Actor {
	return a.scheduler.spawn(creator, opts...)
}

// Proxy 获取代理API
func (a *Actor) Proxy() *Proxy {
	return a.scheduler.node.proxy
}

// Invoke 调用函数（Actor内线程安全）
func (a *Actor) Invoke(fn func()) {
	a.fnChan <- fn
}

// AddRouteHandler 添加路由处理器
func (a *Actor) AddRouteHandler(route int32, handler RouteHandler) {
	a.routes[route] = handler
}

// AddEventHandler 添加事件处理器
func (a *Actor) AddEventHandler(event cluster.Event, handler EventHandler) {
	a.events[event] = handler
}

// Next 投递消息到Actor中进行处理
func (a *Actor) Next(ctx Context) {
	a.mailbox <- ctx
}

// 分发
func (a *Actor) dispatch() {
	go func() {
		for {
			select {
			case ctx, ok := <-a.mailbox:
				if !ok {
					return
				}

				if handler, ok := a.routes[ctx.Route()]; ok {
					xcall.Call(func() { handler(ctx) })
				}
			}
		}
	}()
}
