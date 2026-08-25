package task

import (
	"context"
	"fmt"
	"sync"
)

// token 信号量令牌，用于控制并发数
type token struct{}

// Group 任务组
// 聚合多个任务的错误，支持上下文取消与并发数限制
type Group struct {
	cancel  func(error)
	wg      sync.WaitGroup
	sem     chan token
	errOnce sync.Once
	err     error
}

// WithContext 创建一个新的任务组，与上下文关联
// 首个任务出错或 Wait 返回时，会以该错误作为原因取消上下文
// @param ctx context.Context 父上下文
// @return @1 *Group 任务组
// @return @2 context.Context 派生上下文
func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Group{cancel: cancel}, ctx
}

// SetLimit 设置并发限制
// 并发数需在任务启动前设置；设置为负数表示不限制并发；已有任务运行时修改会触发 panic
// @param n int 最大并发数
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
// 返回首个任务产生的错误，并取消派生上下文
// @return @1 error 首个任务错误，无错误时返回 nil
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}

// Go 执行任务
// 在全局任务池中执行，受并发数限制；任务出错时记录首个错误并取消上下文
// @param f func() error 待执行的任务
func (g *Group) Go(f func() error) {
	if g.sem != nil {
		g.sem <- token{}
	}

	g.add(f)
}

// TryGo 尝试执行任务
// 并发数未满时立即执行并返回 true，否则返回 false
// @param f func() error 待执行的任务
// @return @1 bool 是否成功启动任务
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

// add 添加任务
// @param f func() error 待执行的任务
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

// done 任务完成时调用
// 释放并发令牌并递减等待计数
func (g *Group) done() {
	if g.sem != nil {
		<-g.sem
	}
	g.wg.Done()
}
