// Package queue 提供有界的泛型数据队列与任务队列
//
// 队列内部维护 Opened、Hanged、Closed 三种状态：消费到哨兵数据并调用 Done(true) 后进入 Hanged，
// 表示不再接收新数据，仅允许消费存量数据；Close 会关闭数据通道并释放所有 Wait 等待者。
//
// 注意：Write 与 Close 之间没有内部互斥，调用方必须自行保证二者串行执行（例如共用同一把读写锁），
// 否则可能向已关闭的通道写入而 panic；构造时传入 rw 可复用调用方的读写锁实现该互斥。
package queue

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
)

const (
	Opened = iota // 打开状态，可正常读写
	Hanged        // 挂起状态，已消费到结束信号，仅允许消费存量数据
	Closed        // 关闭状态，数据通道已关闭
)

// Queue 有界泛型队列
// 通过状态机与结束等待组支持"写入结束信号后等待存量数据排空"的优雅关闭语义
type Queue[T any] struct {
	rw      *sync.RWMutex  // 调用方传入的读写锁，用于与 Close 互斥，可为空
	ch      chan T         // 数据通道
	wg      sync.WaitGroup // 队列结束等待组，Close 或结束信号被消费后释放
	size    int32          // 队列容量
	count   atomic.Int32   // 已写入但尚未完成处理的数据量，仅在 timeout > 0 时维护
	state   atomic.Int32   // 队列状态（Opened/Hanged/Closed）
	timeout time.Duration  // 写入超时时间，0 表示不做超时控制
}

// NewQueue 创建队列
// 创建指定容量与写入超时时间的队列，初始状态为 Opened，结束等待组处于未释放状态
// @param size int32 队列容量
// @param timeout time.Duration 写入超时时间，0 表示不做超时控制（写入将阻塞直到写入成功）
// @param rw ...*sync.RWMutex 可选，调用方传入的读写锁；传入后写入过程将持有其读锁，用于与 Close 互斥
// @return @1 *Queue[T] 队列实例
func NewQueue[T any](size int32, timeout time.Duration, rw ...*sync.RWMutex) *Queue[T] {
	q := &Queue[T]{}
	q.ch = make(chan T, size)
	q.wg.Add(1)
	q.size = size
	q.timeout = timeout

	if len(rw) > 0 {
		q.rw = rw[0]
	}

	return q
}

// Write 写入队列
// 写入前校验队列状态：挂起返回 ErrQueueHanged，关闭返回 ErrQueueClosed；
// 当 timeout > 0 且待处理数据量超过队列容量时，非阻塞写入将在 timeout 后返回 ErrWriteTimeout；
// 阻塞写入（block 为 true）或未配置超时（timeout <= 0）时，写入将阻塞直到写入成功
// @param t T 待写入的数据
// @param block ...bool 可选，是否阻塞写入，默认非阻塞（仅在 timeout > 0 时生效）
// @return @1 error 队列挂起、关闭或写入超时时返回的错误
func (q *Queue[T]) Write(t T, block ...bool) (err error) {
	if q.rw != nil {
		q.rw.RLock()
		err = q.write(t, block...)
		q.rw.RUnlock()
	} else {
		err = q.write(t, block...)
	}

	return
}

// write 写入队列
// 执行实际写入逻辑，非阻塞写入且待处理数据量超过队列容量时按超时时间等待写入
// @param t T 待写入的数据
// @param block ...bool 可选，是否阻塞写入，默认非阻塞
// @return @1 error 队列挂起、关闭或写入超时时返回的错误
func (q *Queue[T]) write(t T, block ...bool) error {
	switch q.state.Load() {
	case Hanged:
		return errors.ErrQueueHanged
	case Closed:
		return errors.ErrQueueClosed
	}

	if q.timeout > 0 {
		if q.count.Add(1) > q.size && (len(block) == 0 || !block[0]) {
			timer := time.NewTimer(q.timeout)

			select {
			case q.ch <- t:
				timer.Stop()
				return nil
			case <-timer.C:
				q.count.Add(-1)
				return errors.ErrWriteTimeout
			}
		}
	}

	q.ch <- t

	return nil
}

// Read 读取队列
// 返回队列的数据通道，消费方取出数据后需调用 Done 释放背压计数
// @return @1 <-chan T 队列数据通道
func (q *Queue[T]) Read() <-chan T {
	return q.ch
}

// Done 完成一个数据的处理
// 递减背压计数；isCloseSig 为 true 且队列处于打开状态时，将队列置为挂起并释放队列结束等待；
// 队列已挂起或已关闭时不做任何处理
// @param isCloseSig bool 是否为结束信号（消费到的数据为哨兵数据）
func (q *Queue[T]) Done(isCloseSig bool) {
	if q.state.Load() != Opened {
		return
	}

	if q.timeout > 0 {
		q.count.Add(-1)
	}

	if isCloseSig && q.state.CompareAndSwap(Opened, Hanged) {
		q.wg.Done()
	}
}

// Wait 等待队列完成
// 队列被关闭或结束信号被消费后返回
func (q *Queue[T]) Wait() {
	q.wg.Wait()
}

// Close 关闭队列
// 关闭数据通道并释放队列结束等待，重复调用不会重复关闭通道；
// 调用方必须保证与 Write 串行执行，否则可能向已关闭的通道写入而 panic
func (q *Queue[T]) Close() {
	if q.rw != nil {
		q.rw.Lock()
	}

	switch q.state.Swap(Closed) {
	case Opened:
		q.wg.Done()
		close(q.ch)
	case Hanged:
		close(q.ch)
	}

	if q.rw != nil {
		q.rw.Unlock()
	}
}

// Clean 清理已关闭队列中的所有数据，调用回调函数处理每个数据
// 仅可在队列关闭后调用，否则将阻塞等待新数据
// @param f func(T) 处理函数
func (q *Queue[T]) Clean(f func(T)) {
	for t := range q.ch {
		f(t)
	}
}
