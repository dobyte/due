package queue

import (
	"sync"
	"time"

	"github.com/dobyte/due/v2/utils/xcall"
)

type task struct {
	wg *sync.WaitGroup // Task wait group, nil in the non-waiting mode
	fn func()          // Task function to execute
}

// Tasker is a task runner.
//
// A Tasker owns the asynchronous task queue of a node.
type Tasker struct {
	pool  *sync.Pool    // Task object pool
	queue *Queue[*task] // Task queue
}

// NewTasker returns a new Tasker whose task queue has the given capacity and write timeout. A
// timeout of 0 disables timeout control. The optional rw is passed through to [NewQueue].
func NewTasker(size int32, timeout time.Duration, rw ...*sync.RWMutex) *Tasker {
	return &Tasker{
		pool:  &sync.Pool{New: func() any { return &task{} }},
		queue: NewQueue[*task](size, timeout, rw...),
	}
}

// Commit commits fn as a task. It takes a task object from the pool, stores fn in it and writes it
// to the task queue; when enqueuing fails, the task object is released immediately.
//
// When wait is true, Commit returns a *sync.WaitGroup that the caller can wait on; otherwise it
// returns a nil WaitGroup.
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

// release releases tk. It clears the task's function and wait group and returns the task object to
// the pool; a task holding a wait group releases it immediately so that waiters do not block
// forever.
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

// Read returns the channel from which the tasker's queued tasks are received.
func (t *Tasker) Read() <-chan *task {
	return t.queue.Read()
}

// Clean cancels every unfinished task still in the queue and releases their wait groups. It must
// only be called after the queue has been closed.
func (t *Tasker) Clean() {
	t.queue.Clean(t.release)
}

// Done stops accepting tasks. It writes a sentinel task that tells the consumer the task queue has
// finished; once the consumer handles the sentinel, the queue becomes hanged.
func (t *Tasker) Done() error {
	return t.queue.Write(nil, true)
}

// Wait blocks until the sentinel task has been received or the queue is closed.
func (t *Tasker) Wait() {
	t.queue.Wait()
}

// Close closes the task queue.
func (t *Tasker) Close() {
	t.queue.Close()
}

// Handle handles a queued task. A nil tk is the sentinel that hangs the queue. Handle first
// acknowledges the queue signal, then, when isExecute is true, runs the task function through
// [xcall.Call], and finally returns the task object to the pool.
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
