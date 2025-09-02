package xcall

import (
	"context"
	"sync"
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

	var wg sync.WaitGroup
	wg.Add(len(g.fns))

	for i := range g.fns {
		fn := g.fns[i]
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
