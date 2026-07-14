package node

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

type EventHandler func(ctx Context)

type Trigger struct {
	node   *Node
	rw     sync.RWMutex
	queue  *queue.Queue[*event]
	events map[cluster.Event]EventHandler
}

func newTrigger(node *Node) *Trigger {
	return &Trigger{
		node:   node,
		queue:  queue.NewQueue[*event](node.opts.messageQueueSize, node.opts.messageWriteTimeout),
		events: make(map[cluster.Event]EventHandler, 3),
	}
}

// 触发事件
func (t *Trigger) trigger(kind cluster.Event, gid string, cid, uid int64) error {
	evt := t.node.evtPool.Get().(*event)
	evt.event = kind
	evt.gid = gid
	evt.cid = cid
	evt.uid = uid

	if t.node.opts.ctxFunc != nil {
		evt.ctx = t.node.opts.ctxFunc()
	} else {
		evt.ctx = context.Background()
	}

	t.rw.RLock()
	err := t.queue.Write(evt)
	t.rw.RUnlock()

	return err
}

// 接收事件消息
func (t *Trigger) receive() <-chan *event {
	return t.queue.Read()
}

// 停止接收事件
func (t *Trigger) done() error {
	return t.queue.Write(nil)
}

// 等待所有事件完成
func (t *Trigger) wait() {
	t.queue.Wait()
}

// 关闭事件触发器
func (t *Trigger) close() {
	t.rw.Lock()
	t.queue.Close()
	t.rw.Unlock()

	clear(t.events)
}

// 处理事件消息
func (t *Trigger) handle(evt *event) {
	t.queue.Done(evt == nil)

	if evt == nil {
		return
	}

	version := evt.incrVersion()

	if handler, ok := t.events[evt.event]; ok {
		xcall.Call(func() { handler(evt) })

		evt.compareVersionExecDefer(version)
	}

	evt.compareVersionRecycle(version)
}

// 添加事件处理器
func (t *Trigger) addEventHandler(event cluster.Event, handler EventHandler) {
	if t.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add Event handler")
		return
	}

	t.events[event] = handler
}
