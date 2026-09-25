package queue

import (
	"sync"
	"time"

	"github.com/dobyte/due/v2/utils/xcall"
)

type task struct {
	wg *sync.WaitGroup
	fn func()
}

// Tasker 任务器
// 承载节点内的异步任务队列
type Tasker struct {
	pool  *sync.Pool
	queue *Queue[*task]
}

func NewTasker(size int32, timeout time.Duration, rw ...*sync.RWMutex) *Tasker {
	return &Tasker{
		pool:  &sync.Pool{New: func() any { return &task{} }},
		queue: NewQueue[*task](size, timeout, rw...),
	}
}

// 写入任务消息
// @param fn func() 待执行的任务函数
// @param wait ...bool 是否等待任务完成
// @return @1 *sync.WaitGroup 任务等待组
// @return @2 error 任务入队失败时返回的错误
func (t *Tasker) Commit(fn func(), wait ...bool) (*sync.WaitGroup, error) {
	tk := t.pool.Get().(*task)
	tk.fn = fn

	var wg *sync.WaitGroup
	if len(wait) > 0 && wait[0] {
		wg = &sync.WaitGroup{}
		wg.Add(1)
		tk.wg = wg
	}

	if err := t.queue.Write(tk); err != nil {
		t.release(tk)
		return nil, err
	}

	return wg, nil
}

// 释放任务
// @param tk *task 待释放的任务
func (t *Tasker) release(tk *task) {
	if tk == nil {
		return
	}

	tk.fn = nil

	if tk.wg != nil {
		tk.wg.Done()
		tk.wg = nil
	}

	t.pool.Put(tk)
}

// 接收任务消息
// @return @1 <-chan *task 任务消息通道
func (t *Tasker) Read() <-chan *task {
	return t.queue.Read()
}

// 清理任务队列
// 取消所有未完成的任务
func (t *Tasker) Clean() {
	t.queue.Clean(t.release)
}

// 停止接收任务
// 写入空任务以通知分发器任务队列已结束
// @return @1 error 写入失败时返回的错
func (t *Tasker) Done() error {
	return t.queue.Write(nil, true)
}

// 等待所有任务完成
func (t *Tasker) Wait() {
	t.queue.Wait()
}

// 关闭任务器
func (t *Tasker) Close() {
	t.queue.Close()
}

// 处理任务消息
// 安全执行任务函数并在完成后递减节点等待计数
// @param tk *task 待处理的任务
func (t *Tasker) Handle(tk *task) {
	t.queue.Done(tk == nil)

	if tk == nil {
		return
	}

	xcall.Call(tk.fn)

	t.release(tk)
}
