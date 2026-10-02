package node

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// EventHandler is an event handler function.
type EventHandler func(ctx Context)

// Trigger is an event trigger.
type Trigger struct {
	node   *Node
	queue  *queue.Queue[*event]
	events map[cluster.Event]EventHandler
}

// newTrigger creates a new event trigger for the given node server.
func newTrigger(node *Node) *Trigger {
	return &Trigger{
		node:   node,
		queue:  queue.NewQueue[*event](node.opts.messageQueueSize, node.opts.messageWriteTimeout, &sync.RWMutex{}),
		events: make(map[cluster.Event]EventHandler, 3),
	}
}

// trigger triggers an event. It fetches an event object from the pool, fills it in and writes it to
// the event queue for asynchronous handling. It returns the error reported when the event fails to
// be enqueued.
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

	if err := t.queue.Write(evt); err != nil {
		evt.release()
		return err
	}

	return nil
}

// receive returns the channel from which event messages are received.
func (t *Trigger) receive() <-chan *event {
	return t.queue.Read()
}

// close closes the event trigger.
func (t *Trigger) close() {
	t.queue.Close()
}

// clean releases every event object still in the event queue.
func (t *Trigger) clean() {
	t.queue.Clean(func(evt *event) { evt.release() })
}

// handle handles an event message. It looks up the matching event handler and executes it, then
// recycles the event object once handling is done.
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

// addEventHandler adds an event handler for the given event type.
func (t *Trigger) addEventHandler(event cluster.Event, handler EventHandler) {
	if t.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add Event handler")
		return
	}

	t.events[event] = handler
}
