package ws

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
)

const (
	queueOpened = iota
	queueHanged
	queueClosed
)

type queue struct {
	wg      sync.WaitGroup
	list    chan *task
	count   atomic.Int32
	state   atomic.Int32
	timeout time.Duration
}

func newQueue(size int, timeout time.Duration) *queue {
	q := &queue{}
	q.wg.Add(1)
	q.list = make(chan *task, size)
	q.timeout = timeout

	return q
}

// write 写入队列
func (q *queue) write(t *task) error {
	switch q.state.Load() {
	case queueOpened:
		// write success
	case queueHanged:
		return errors.ErrQueueHanged
	case queueClosed:
		return errors.ErrQueueClosed
	}

	var (
		err           error
		isWithTimeout bool
	)

	if q.timeout > 0 {
		isWithTimeout = q.count.Add(1) > int32(cap(q.list))
	}

	defer func() {
		if err != nil && q.timeout > 0 {
			q.count.Add(-1)
		}
	}()

	if isWithTimeout {
		ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
		defer cancel()

		select {
		case q.list <- t:
			// write success
		case <-ctx.Done():
			err = errors.ErrWriteTimeout
		}
	} else {
		select {
		case q.list <- t:
			// write success
		default:
			err = errors.ErrWriteTimeout
		}
	}

	return err
}

// 读取队列
func (q *queue) read() chan *task {
	return q.list
}

// 完成一个任务
func (q *queue) done(isCloseSig bool) {
	if q.state.Load() != queueOpened {
		return
	}

	if q.timeout > 0 {
		q.count.Add(-1)
	}

	if isCloseSig && q.state.CompareAndSwap(queueOpened, queueHanged) {
		q.wg.Done()
	}
}

// 等待队列完成
func (q *queue) wait() {
	q.wg.Wait()
}

// 关闭队列
func (q *queue) close() {
	switch q.state.Swap(queueClosed) {
	case queueOpened:
		q.wg.Done()
		close(q.list)
	case queueHanged:
		close(q.list)
	}
}
