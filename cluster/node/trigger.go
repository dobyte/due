package node

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// EventHandler 事件处理函数
type EventHandler func(ctx Context)

// Trigger 事件触发器
type Trigger struct {
	node   *Node
	rw     sync.RWMutex
	queue  *queue.Queue[*event]
	events map[cluster.Event]EventHandler
}

// 创建事件触发器
// @param node *Node 节点服务器
// @return @1 *Trigger 事件触发器
func newTrigger(node *Node) *Trigger {
	return &Trigger{
		node:   node,
		queue:  queue.NewQueue[*event](node.opts.messageQueueSize, node.opts.messageWriteTimeout),
		events: make(map[cluster.Event]EventHandler, 3),
	}
}

// 触发事件
// 从对象池获取事件对象填充后写入事件队列等待异步处理
// @param kind cluster.Event 事件类型
// @param gid string 网关ID
// @param cid int64 连接ID
// @param uid int64 用户ID
// @return @1 error 事件入队失败时返回的错误
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

	if err != nil {
		evt.release()
		return err
	}

	return nil
}

// 接收事件消息
// @return @1 <-chan *event 事件消息通道
func (t *Trigger) receive() <-chan *event {
	return t.queue.Read()
}

// 停止接收事件
// 写入空事件以通知分发器事件队列已结束
// @return @1 error 写入失败时返回的错误
func (t *Trigger) done() error {
	return t.queue.Write(nil)
}

// 等待所有事件完成
func (t *Trigger) wait() {
	t.queue.Wait()
}

// 关闭事件触发器
// 关闭事件队列并清空已注册的事件处理器
func (t *Trigger) close() {
	t.rw.Lock()
	t.queue.Close()
	t.rw.Unlock()

	clear(t.events)
}

// 处理事件消息
// 查找对应事件处理器并执行，处理完成后回收事件对象
// @param evt *event 事件对象
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
// @param event cluster.Event 事件类型
// @param handler EventHandler 事件处理函数
func (t *Trigger) addEventHandler(event cluster.Event, handler EventHandler) {
	if t.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add Event handler")
		return
	}

	t.events[event] = handler
}
