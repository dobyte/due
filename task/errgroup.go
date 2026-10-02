package task

import (
	"context"
	"fmt"
	"sync"
)

// token is a semaphore token used to control concurrency.
type token struct{}

// Group aggregates the errors of multiple tasks and supports context cancellation and a
// concurrency limit.
type Group struct {
	cancel  func(error)
	wg      sync.WaitGroup
	sem     chan token
	errOnce sync.Once
	err     error
}

// WithContext returns a new Group and a context derived from ctx.
//
// The derived context is canceled with the first task error as its cause when the first task fails
// or when Wait returns.
func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Group{cancel: cancel}, ctx
}

// SetLimit sets the concurrency limit to n.
//
// The limit must be set before any task is started. A negative n means no limit. Changing the
// limit while tasks are running panics.
func (g *Group) SetLimit(n int) {
	if n < 0 {
		g.sem = nil
		return
	}
	if active := len(g.sem); active != 0 {
		panic(fmt.Errorf("errgroup: modify limit while %v goroutines in the group are still active", active))
	}
	g.sem = make(chan token, n)
}

// Wait blocks until all tasks have finished and cancels the derived context. It returns the error
// of the first failing task, or nil when no task failed.
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}

// Go runs f as a task.
//
// The task runs in the global task pool and is subject to the concurrency limit. When the task
// fails, its error is recorded if it is the first one and the context is canceled.
func (g *Group) Go(f func() error) {
	if g.sem != nil {
		g.sem <- token{}
	}

	g.add(f)
}

// TryGo tries to run f as a task.
//
// It starts the task immediately and reports true when the concurrency limit has not been reached;
// otherwise it reports false.
func (g *Group) TryGo(f func() error) bool {
	if g.sem != nil {
		select {
		case g.sem <- token{}:
			// Note: this allows barging iff channels in general allow barging.
		default:
			return false
		}
	}

	g.add(f)

	return true
}

// add adds f as a task.
func (g *Group) add(f func() error) {
	g.wg.Add(1)
	Add(func() {
		defer g.done()

		if err := f(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel(g.err)
				}
			})
		}
	})
}

// done is called when a task finishes. It releases the concurrency token and decrements the wait
// count.
func (g *Group) done() {
	if g.sem != nil {
		<-g.sem
	}
	g.wg.Done()
}
