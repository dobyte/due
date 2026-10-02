// Package queue provides a bounded generic data queue and a task queue.
//
// A queue moves through three states: [Opened], [Hanged] and [Closed]. Once the sentinel data has
// been received and [Queue.Done] is called with true, the queue becomes [Hanged]: it stops accepting
// new data and only the remaining data may be received. [Queue.Close] closes the data channel and
// releases every caller blocked in [Queue.Wait].
//
// Write and Close are not mutually exclusive internally, so callers must serialize them, for
// example by sharing the same read-write lock; otherwise a write to an already closed channel
// panics. Passing rw to [NewQueue] makes the queue reuse the caller's read-write lock to provide
// that mutual exclusion.
package queue

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
)

// The states of a [Queue].
const (
	Opened = iota // Opened: both reads and writes are allowed
	Hanged        // Hanged: the end signal has been received, only the remaining data may be received
	Closed        // Closed: the data channel has been closed
)

// Queue is a bounded generic queue.
//
// Queue uses a state machine and an end wait group to support the graceful shutdown semantics of
// "write the end signal, then wait for the remaining data to drain".
type Queue[T any] struct {
	rw      *sync.RWMutex  // Read-write lock supplied by the caller to serialize writes against Close, may be nil
	ch      chan T         // Data channel
	wg      sync.WaitGroup // End wait group, released by Close or after the end signal has been received
	size    int32          // Queue capacity
	count   atomic.Int32   // Amount of data written but not yet processed, maintained only when timeout > 0
	state   atomic.Int32   // Current queue state, one of Opened, Hanged or Closed
	timeout time.Duration  // Write timeout, 0 means no timeout control
}

// NewQueue returns a new Queue with the given capacity and write timeout. The timeout bounds how
// long a non-blocking [Queue.Write] waits when the queue is full; a timeout of 0 disables that
// bound and makes writes block until they succeed.
//
// The optional rw is a read-write lock supplied by the caller. When it is provided, writes hold its
// read lock so that they are serialized against [Queue.Close].
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

// Write writes t to the queue. It reports [errors.ErrQueueHanged] when the queue is hanged and
// [errors.ErrQueueClosed] when it is closed.
//
// When the write timeout is greater than 0 and the amount of pending data exceeds the queue
// capacity, a non-blocking write waits for at most the timeout and then reports
// [errors.ErrWriteTimeout]. Pass block as true, or configure a timeout <= 0, to block until the
// write succeeds.
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

// write writes t to the queue. For a non-blocking write whose pending data exceeds the queue
// capacity, it waits for the write timeout before giving up.
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

// Read returns the channel from which the queue's data is received. After receiving an item, the
// consumer must call [Queue.Done] to release the backpressure counter.
func (q *Queue[T]) Read() <-chan T {
	return q.ch
}

// Done acknowledges that one item of the queue's data has been processed. It decrements the
// backpressure counter and, when isCloseSig is true and the queue is in the [Opened] state, moves
// the queue to [Hanged] and releases the end wait. Done does nothing when the queue is already
// hanged or closed.
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

// Wait blocks until the queue is closed or the end signal has been received.
func (q *Queue[T]) Wait() {
	q.wg.Wait()
}

// Close closes the queue. It closes the data channel and releases the end wait; repeated calls do
// not close the channel twice. The caller must serialize Close with [Queue.Write], otherwise a
// write to the already closed channel panics.
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

// Clean receives and discards every remaining item of a closed queue, invoking f for each item. It
// must only be called after the queue has been closed; otherwise it blocks waiting for new data.
func (q *Queue[T]) Clean(f func(T)) {
	for t := range q.ch {
		f(t)
	}
}
