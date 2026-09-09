package xcall

import (
	"context"
	"sync"
	"time"
)

// Goroutines 协程组，用于批量管理需要并发执行的协程函数
type Goroutines struct {
	mu  sync.Mutex
	fns []func()
}

// NewGoroutines 创建一个协程组实例
// @return @1 *Goroutines 协程组实例
func NewGoroutines() *Goroutines {
	return &Goroutines{}
}

// Add 添加协程函数
// @param fns ...func() 待添加的协程函数
// @return @1 *Goroutines 协程组实例（支持链式调用）
func (g *Goroutines) Add(fns ...func()) *Goroutines {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.fns = append(g.fns, fns...)

	return g
}

// Run 运行协程函数，所有函数在独立的协程中并发执行
// 执行完毕后会清空已添加的协程函数，以便实例可被复用
// 未指定超时时间时会等待所有协程执行完毕；
// 指定超时时间后，若超时则提前返回，不再等待剩余协程
// @param ctx context.Context 上下文
// @param timeout ...time.Duration 可选，整体执行的超时时间
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
