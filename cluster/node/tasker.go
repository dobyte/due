package node

import (
	"sync"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/utils/xcall"
)

// Tasker 任务器
// 承载节点内的异步任务队列
type Tasker struct {
	node  *Node
	rw    sync.RWMutex
	queue *queue.Queue[func()]
}

// 创建任务器
// @param node *Node 节点服务器
// @return @1 *Tasker 任务器
func newTasker(node *Node) *Tasker {
	return &Tasker{
		node:  node,
		queue: queue.NewQueue[func()](node.opts.taskQueueSize, node.opts.taskWriteTimeout),
	}
}

// 写入任务消息
// @param f func() 待执行的任务函数
// @return @1 error 任务入队失败时返回的错误
func (t *Tasker) commit(f func()) error {
	t.rw.RLock()
	err := t.queue.Write(f)
	t.rw.RUnlock()

	return err
}

// 接收任务消息
// @return @1 <-chan func() 任务消息通道
func (t *Tasker) receive() <-chan func() {
	return t.queue.Read()
}

// 停止接收任务
// 写入空任务以通知分发器任务队列已结束
// @return @1 error 写入失败时返回的错误
func (t *Tasker) done() error {
	return t.queue.Write(nil)
}

// 等待所有任务完成
func (t *Tasker) wait() {
	t.queue.Wait()
}

// 关闭任务器
// 关闭任务队列
func (t *Tasker) close() {
	t.rw.Lock()
	t.queue.Close()
	t.rw.Unlock()
}

// 处理任务消息
// 安全执行任务函数并在完成后递减节点等待计数
// @param f func() 任务函数
func (t *Tasker) handle(f func()) {
	t.queue.Done(f == nil)

	if f == nil {
		return
	}

	xcall.Call(f)

	t.node.doDoneWait()
}
