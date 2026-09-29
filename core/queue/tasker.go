package queue

import (
	"sync"
	"time"

	"github.com/dobyte/due/v2/utils/xcall"
)

type task struct {
	wg *sync.WaitGroup // 任务等待组，非等待模式为空
	fn func()          // 待执行的任务函数
}

// Tasker 任务器
// 承载节点内的异步任务队列
type Tasker struct {
	pool  *sync.Pool    // 任务对象池
	queue *Queue[*task] // 任务队列
}

// NewTasker 创建任务器
// @param size int32 任务队列容量
// @param timeout time.Duration 任务写入超时时间，0 表示不做超时控制
// @param rw ...*sync.RWMutex 可选，调用方传入的读写锁，用于与 Close 互斥
// @return @1 *Tasker 任务器实例
func NewTasker(size int32, timeout time.Duration, rw ...*sync.RWMutex) *Tasker {
	return &Tasker{
		pool:  &sync.Pool{New: func() any { return &task{} }},
		queue: NewQueue[*task](size, timeout, rw...),
	}
}

// Commit 提交任务
// 从对象池取出任务对象后填充任务函数并写入任务队列，入队失败时同步释放任务对象
// @param fn func() 待执行的任务函数
// @param wait ...bool 可选，是否需要等待任务完成，默认不等待
// @return @1 *sync.WaitGroup 任务等待组，非等待模式下返回nil
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

// release 释放任务
// 清空任务引用与等待组并归还对象池；任务持有等待组时同步释放，避免等待方永久阻塞
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

// Read 读取任务队列
// @return @1 <-chan *task 任务数据通道
func (t *Tasker) Read() <-chan *task {
	return t.queue.Read()
}

// Clean 清理任务队列
// 取消所有未完成的任务并释放其等待组，仅可在队列关闭后调用
func (t *Tasker) Clean() {
	t.queue.Clean(t.release)
}

// Done 停止接收任务
// 写入哨兵任务以通知消费方任务队列已结束，消费方处理哨兵后队列进入挂起状态
// @return @1 error 写入失败时返回的错误
func (t *Tasker) Done() error {
	return t.queue.Write(nil, true)
}

// Wait 等待所有任务完成
// 哨兵任务被消费或队列关闭后返回
func (t *Tasker) Wait() {
	t.queue.Wait()
}

// Close 关闭任务队列
func (t *Tasker) Close() {
	t.queue.Close()
}

// Handle 处理任务
// 先确认队列信号（哨兵任务触发队列挂起），再安全执行任务函数并在执行完成后归还任务对象
// @param tk *task 待处理的任务，为nil表示结束信号
func (t *Tasker) Handle(tk *task, isExecute bool) {
	t.queue.Done(tk == nil)

	if tk == nil {
		return
	}

	if isExecute {
		xcall.Call(tk.fn)
	}

	t.release(tk)
}
