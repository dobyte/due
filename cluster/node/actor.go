package node

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/utils/xcall"
)

type Actor interface {
	// ID 获取Actor编号
	ID() string
	// PID 获取Actor全局唯一识别号，实际为 Kind/ID
	PID() string
	// Kind 获取Actor类型
	Kind() string
	// Args 获取Actor参数
	Args() []any
	// Next 投递消息到Actor中进行处理
	Next(ctx Context)
	// Spawn 衍生出一个Actor
	Spawn(creator Creator, opts ...ActorOption) Actor
	// Proxy 获取代理API
	Proxy() *Proxy
	// Invoke 调用函数（Actor内线程安全）
	Invoke(fn func())
	// AddRouteHandler 添加路由处理器
	AddRouteHandler(route int32, handler RouteHandler)
	// AddEventHandler 添加事件处理器
	AddEventHandler(event cluster.Event, handler EventHandler)
}

type Creator func(actor Actor, args ...any) Processor

type actor struct {
	opts      *actorOptions
	proxy     *Proxy
	routes    map[int32]RouteHandler         // 路由处理器
	events    map[cluster.Event]EventHandler // 事件处理器
	processor Processor                      // 处理器
	mailbox   chan Context                   // 邮箱
	fnChan    chan func()
}

// ID 获取Actor的ID
func (a *actor) ID() string {
	return a.opts.id
}

func (a *actor) PID() string {
	return a.Kind() + "/" + a.ID()
}

// Kind 获取Actor类型
func (a *actor) Kind() string {
	return a.processor.Kind()
}

// Args 获取Actor参数
func (a *actor) Args() []any {
	return a.opts.args
}

// Spawn 衍生出一个Actor
func (a *actor) Spawn(creator Creator, opts ...ActorOption) Actor {
	return a.proxy.Spawn(creator, opts...)
}

// Proxy 获取代理API
func (a *actor) Proxy() *Proxy {
	return a.proxy
}

// Invoke 调用函数（Actor内线程安全）
func (a *actor) Invoke(fn func()) {
	a.fnChan <- fn
}

// AddRouteHandler 添加路由处理器
func (a *actor) AddRouteHandler(route int32, handler RouteHandler) {
	a.routes[route] = handler
}

// AddEventHandler 添加事件处理器
func (a *actor) AddEventHandler(event cluster.Event, handler EventHandler) {
	a.events[event] = handler
}

// Next 投递消息到Actor中进行处理
func (a *actor) Next(ctx Context) {
	a.mailbox <- ctx
}

// 分发
func (a *actor) dispatch() {
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
