package node

import "github.com/dobyte/due/v2/cluster"

type baseActor struct {
	fnChan    chan func() // 调用函数
	processor Processor   // 处理器
}

// Kind 获取Actor类型
func (b *baseActor) Kind() string {
	return b.processor.Kind()
}

// Invoke 调用函数（Actor内线程安全）
func (b *baseActor) Invoke(fn func()) {
	b.fnChan <- fn
}

// Spawn 衍生出一个Actor
func (b *baseActor) Spawn(creator Creator, args ...any) Actor {
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
