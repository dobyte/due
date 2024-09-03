package actor

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/utils/xcall"
)

type Creator func(actor *Actor) Processor

type Actor struct {
	opts      *options
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

// PID 获取Actor全局唯一识别号，实际为 Kind/ID
func (a *Actor) PID() string {
	return a.Kind() + "/" + a.opts.id
}

// Kind 获取Actor类型
func (a *Actor) Kind() string {
	return a.processor.Kind()
}

// Spawn 衍生出一个Actor
//func (a *Actor) Spawn(creator Creator, opts ...Option) Actor {
//	o := defaultOptions()
//	for _, opt := range opts {
//		opt(o)
//	}
//
//	act := &actor{}
//	act.opts = o
//	//act.routes = make(map[int32]RouteHandler)
//	//act.events = make(map[cluster.Event]EventHandler, 3)
//	act.mailbox = make(chan Context, 4096)
//	act.processor = creator(act)
//	act.processor.Init()
//	//act.dispatch()
//	act.processor.Start()
//
//	return act
//}

func (a *Actor) dispatch() {
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
}
