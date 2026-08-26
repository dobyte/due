package node

import (
	"sync"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/utils/xcall"
)

type Tasker struct {
	node  *Node
	rw    sync.RWMutex
	queue *queue.Queue[func()]
}

func newTasker(node *Node) *Tasker {
	return &Tasker{
		node:  node,
		queue: queue.NewQueue[func()](node.opts.taskQueueSize, node.opts.taskWriteTimeout),
	}
}

// 写入任务消息
func (t *Tasker) commit(f func()) error {
	t.rw.RLock()
	err := t.queue.Write(f)
	t.rw.RUnlock()

	return err
}

// 接收任务消息
func (t *Tasker) receive() <-chan func() {
	return t.queue.Read()
}

// 停止接收任务
func (t *Tasker) done() error {
	return t.queue.Write(nil)
}

// 等待所有任务完成
func (t *Tasker) wait() {
	t.queue.Wait()
}

// 关闭任务器
func (t *Tasker) close() {
	t.rw.Lock()
	t.queue.Close()
	t.rw.Unlock()
}

// 处理任务消息
func (t *Tasker) handle(f func()) {
	t.queue.Done(f == nil)

	if f == nil {
		return
	}

	xcall.Call(f)

	t.node.doDoneWait()
}
