package task

import (
	"context"
	"fmt"
	"sync"
)

type token struct{}

type Group struct {
	cancel  func(error)
	wg      sync.WaitGroup
	sem     chan token
	errOnce sync.Once
	err     error
}

// WithContext 创建一个新的任务组，与上下文关联
func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Group{cancel: cancel}, ctx
}

// SetLimit 设置并发限制
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

// Wait 等待所有任务完成
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}

// Go 执行任务
func (g *Group) Go(f func() error) {
	if g.sem != nil {
		g.sem <- token{}
	}

	g.add(f)
}

// TryGo 尝试执行任务
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

// 添加任务
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

// 任务完成时调用
func (g *Group) done() {
	if g.sem != nil {
		<-g.sem
	}
	g.wg.Done()
}
