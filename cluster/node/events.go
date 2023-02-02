package node

import (
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/log"
	"sync"
)

type EventHandler func(gid string, uid int64)

type Event struct {
	event cluster.Event
	gid   string
	cid   int64
	uid   int64
}

type Events struct {
	node    *Node
	events  map[cluster.Event]EventHandler
	evtPool sync.Pool
	chEvent chan *Event
}

func newEvents(node *Node) *Events {
	return &Events{
		node:    node,
		events:  make(map[cluster.Event]EventHandler, 3),
		evtPool: sync.Pool{New: func() interface{} { return &Event{} }},
		chEvent: make(chan *Event, 4096),
	}
}

// 处理事件
func (e *Events) handle(evt *Event) {
	defer e.evtPool.Put(evt)

	handler, ok := e.events[evt.event]
	if !ok {
		log.Warnf("event does not register handler function, event: %v", evt.event)
		return
	}

	handler(evt.gid, evt.uid)
}

// 触发事件
func (e *Events) trigger(event cluster.Event, gid string, cid, uid int64) {
	evt := e.evtPool.Get().(*Event)
	evt.event = event
	evt.gid = gid
	evt.cid = cid
	evt.uid = uid
	e.chEvent <- evt
}

func (e *Events) event() <-chan *Event {
	return e.chEvent
}

func (e *Events) close() {
	close(e.chEvent)
}

// AddEventListener 添加事件处理器
func (e *Events) AddEventListener(event cluster.Event, handler EventHandler) {
	if e.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add event handler")
		return
	}

	e.events[event] = handler
}
