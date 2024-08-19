package node

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/utils/xcall"
)

type Actor interface {
	// Kind 获取Actor类型
	Kind() string
	// Spawn 衍生出一个Actor
	Spawn(creator Creator, args ...any) Actor
	// Proxy 获取代理API
	Proxy() *Proxy
	// Invoke 调用函数（Actor内线程安全）
	Invoke(fn func())
	// AddRouteHandler 添加路由处理器
	AddRouteHandler(route int32, handler RouteHandler)
	// AddEventHandler 添加事件处理器
	AddEventHandler(event cluster.Event, handler EventHandler)
	// 投递消息到Actor中进行处理
	deliver(ctx Context)
}

type Creator func(actor Actor, args ...any) Processor

type actor struct {
	routes    map[int32]RouteHandler         // 路由处理器
	events    map[cluster.Event]EventHandler // 事件处理器
	processor Processor                      // 处理器
	mailbox   chan Context                   // 邮箱
	fnChan    chan func()
}

// Kind 获取Actor类型
func (a *actor) Kind() string {
	return a.processor.Kind()
}

// Spawn 衍生出一个Actor
func (a *actor) Spawn(creator Creator, args ...any) Actor {
	act := &actor{}
	act.routes = make(map[int32]RouteHandler)
	act.events = make(map[cluster.Event]EventHandler, 3)
	act.mailbox = make(chan Context, 4096)
	act.processor = creator(act, args...)
	act.processor.Init()
	act.dispatch()
	act.processor.Start()

	return act
}

// Proxy 获取代理API
func (a *actor) Proxy() *Proxy {
	return nil
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

// 投递消息到Actor中进行处理
func (a *actor) deliver(ctx Context) {
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
