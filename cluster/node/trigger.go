package node

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/log"
	"sync"
)

type EventHandler func(ctx Context)

type Trigger struct {
	node    *Node
	events  map[cluster.Event]EventHandler
	evtPool sync.Pool
	evtChan chan *event
}

func newTrigger(node *Node) *Trigger {
	return &Trigger{
		node:    node,
		events:  make(map[cluster.Event]EventHandler, 3),
		evtPool: sync.Pool{New: func() interface{} { return &event{proxy: node.proxy} }},
		evtChan: make(chan *event, 4096),
	}
}

func (e *Trigger) trigger(kind cluster.Event, gid string, cid, uid int64) {
	evt := e.evtPool.Get().(*event)
	evt.kind = kind
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

func (e *Trigger) handle(evt *event) {
	defer e.evtPool.Put(evt)

	handler, ok := e.events[evt.kind]
	if !ok {
		return
	}

	handler(evt)
}

// AddEventHandler 添加事件处理器
func (e *Trigger) AddEventHandler(event cluster.Event, handler EventHandler) {
	if e.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add Event handler")
		return
	}

	e.events[event] = handler
}
