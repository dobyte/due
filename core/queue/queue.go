package queue

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
)

const (
	Opened = iota
	Hanged
	Closed
)

type Queue[T any] struct {
	rw      *sync.RWMutex
	ch      chan T
	wg      sync.WaitGroup
	size    int32
	count   atomic.Int32
	state   atomic.Int32
	timeout time.Duration
}

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
// @param t T 待写入的消息
// @param block 是否阻塞写入，默认非阻塞
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
// @param t T 待写入的消息
// @param block 是否阻塞写入，默认非阻塞
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
func (q *Queue[T]) Read() <-chan T {
	return q.ch
}

// Done 完成一个任务
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
func (q *Queue[T]) Wait() {
	q.wg.Wait()
}

// Close 关闭队列
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

// Clean 清理已关闭队列中的所有消息，调用回调函数处理每个消息
// @param f 处理函数
func (q *Queue[T]) Clean(f func(T)) {
	for t := range q.ch {
		f(t)
	}
}
