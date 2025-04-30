package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

type EventHandler func(ctx Context)

type Trigger struct {
	node    *Node
	events  map[cluster.Event]EventHandler
	evtChan chan *event
}

func newTrigger(node *Node) *Trigger {
	return &Trigger{
		node:    node,
		events:  make(map[cluster.Event]EventHandler, 3),
		evtChan: make(chan *event, 4096),
	}
}

func (e *Trigger) trigger(kind cluster.Event, gid string, cid, uid int64) {
	evt := e.node.evtPool.Get().(*event)
	evt.ctx = context.Background()
	evt.event = kind
	evt.gid = gid
	evt.cid = cid
	evt.uid = uid
	e.evtChan <- evt
}

func (e *Trigger) receive() <-chan *event {
	return e.evtChan
}

func (e *Trigger) close() {
	close(e.evtChan)
}

// 处理事件消息
func (e *Trigger) handle(evt *event) {
	version := evt.incrVersion()

	if handler, ok := e.events[evt.event]; ok {
		xcall.Call(func() { handler(evt) })

		evt.compareVersionExecDefer(version)
	}

	evt.compareVersionRecycle(version)
}

// AddEventHandler 添加事件处理器
func (e *Trigger) AddEventHandler(event cluster.Event, handler EventHandler) {
	if e.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add Event handler")
		return
	}

	e.events[event] = handler
}
