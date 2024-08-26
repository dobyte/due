package actor

import "github.com/dobyte/due/v2/cluster"

type Actor interface {
}

type Creator func(actor Actor, args ...any) Processor

type actor struct {
	opts      *options
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

// PID 获取Actor全局唯一识别号，实际为 Kind/ID
func (a *actor) PID() string {
	return a.opts.kind + "/" + a.opts.id
}

// Kind 获取Actor类型
func (a *actor) Kind() string {
	return a.opts.kind
}

// Spawn 衍生出一个Actor
func (a *actor) Spawn(creator Creator, opts ...Option) Actor {
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
