package queue

import (
	"context"
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
	ch      chan T
	wg      sync.WaitGroup
	size    int32
	count   atomic.Int32
	state   atomic.Int32
	timeout time.Duration
}

func NewQueue[T any](size int32, timeout time.Duration) *Queue[T] {
	q := &Queue[T]{}
	q.ch = make(chan T, size)
	q.wg.Add(1)
	q.size = size
	q.timeout = timeout

	return q
}

// Write 写入队列
func (q *Queue[T]) Write(t T) error {
	switch q.state.Load() {
	case Hanged:
		return errors.ErrQueueHanged
	case Closed:
		return errors.ErrQueueClosed
	}

	if q.timeout > 0 && q.count.Add(1) > q.size {
		ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
		defer cancel()

		select {
		case q.ch <- t:
			return nil
		case <-ctx.Done():
			if q.timeout > 0 {
				q.count.Add(-1)
			}

			return errors.ErrWriteTimeout
		}
	} else {
		q.ch <- t
	}

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
	switch q.state.Swap(Closed) {
	case Opened:
		q.wg.Done()
		close(q.ch)
	case Hanged:
		close(q.ch)
	}
}
