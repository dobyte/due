package xcall

import (
	"context"
	"sync"
	"time"
)

// Goroutines is a goroutine group that manages a batch of functions to run concurrently.
type Goroutines struct {
	mu  sync.Mutex
	fns []func()
}

// NewGoroutines returns a new Goroutines group.
func NewGoroutines() *Goroutines {
	return &Goroutines{}
}

// Add appends the given functions to the group and returns the group itself so that calls can be
// chained.
func (g *Goroutines) Add(fns ...func()) *Goroutines {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.fns = append(g.fns, fns...)

	return g
}

// Run starts every function the group holds, each in its own goroutine, and waits for them
// according to ctx.
//
// The group is emptied before the functions run, so the same instance can be reused. When no
// timeout is given, Run blocks until all functions have finished; when a timeout is given, it
// returns early on timeout without waiting for the remaining functions.
func (g *Goroutines) Run(ctx context.Context, timeout ...time.Duration) {
	g.mu.Lock()
	fns := g.fns
	g.fns = nil
	g.mu.Unlock()

	if len(fns) == 0 {
		return
	}

	if len(timeout) > 0 && timeout[0] > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout[0])
		defer cancel()
	}

	var wg sync.WaitGroup
	wg.Add(len(fns))

	for i := range fns {
		fn := fns[i]
		Go(func() {
			defer wg.Done()
			fn()
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}
}
