package xcall

import (
	"context"
	"sync/atomic"
	"time"
)

type Goroutines struct {
	fns []func()
}

func NewGoroutines() *Goroutines {
	return &Goroutines{}
}

// Add 添加协程函数
func (g *Goroutines) Add(fns ...func()) *Goroutines {
	g.fns = append(g.fns, fns...)
	return g
}

// Run 运行协程函数
func (g *Goroutines) Run(ctx context.Context, timeout ...time.Duration) {
	if len(g.fns) == 0 {
		return
	}

	if len(timeout) > 0 && timeout[0] > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout[0])
		defer cancel()
	}

	done := make(chan struct{})
	over := make(chan struct{})
	defer close(done)
	defer close(over)

	go func() {
		var total atomic.Int32
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-done:
				if !ok {
					return
				}

				if int(total.Add(1)) == len(g.fns) {
					over <- struct{}{}
					return
				}
			}
		}
	}()

	for i := range g.fns {
		fn := g.fns[i]
		Go(func() {
			fn()
			done <- struct{}{}
		})
	}

	select {
	case <-ctx.Done():
	case <-over:
	}
}
