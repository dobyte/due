package xcall

import (
	"context"
	"runtime"
	"time"

	"github.com/dobyte/due/v2/log"
)

type funcType interface {
	func() | func() error
}

// Call 安全地调用函数
// 捕获函数执行过程中产生的 panic：运行时错误（runtime.Error）记录为致命错误，其他 panic 记录错误信息
// @param fn func() 待调用的函数
func Call[T funcType](fn T) error {
	if fn == nil {
		return nil
	}

	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case runtime.Error:
				log.Panic(err)
			default:
				log.Panicf("panic error: %v", err)
			}
		}
	}()

	switch f := any(fn).(type) {
	case func():
		f()
	case func() error:
		return f()
	}

	return nil
}

// Go 执行单个协程
// 将函数放入新的协程中执行，并自动捕获 panic，避免协程崩溃导致整个进程退出
// @param fn func() 待执行的函数
func Go[T funcType](fn T) {
	go Call(fn)
}

// Backoff 指数退避调用函数
// 按指数递增的间隔重试调用 fn：第 i 次尝试的间隔为 1<<i 倍的基础延迟（封顶为 maxDelay），
// 直到 fn 返回 next=false、ctx 被取消或达到最大重试次数 retry
// @param ctx context.Context 上下文
// @param fn func(ctx context.Context, attempt int) (bool, error) 待调用的函数，attempt 为当前尝试次数（从1开始），返回值 next 表示是否继续重试
// @param retry int 最大重试次数
// @param baseDelay time.Duration 基础延迟时间
// @param maxDelay time.Duration 最大延迟时间
// @return @1 error 最后一次调用返回的错误；若 ctx 被取消，则返回 ctx.Err()
func Backoff(ctx context.Context, fn func(ctx context.Context, attempt int) (bool, error), retry int, baseDelay, maxDelay time.Duration) error {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case runtime.Error:
				log.Panic(err)
			default:
				log.Panicf("panic error: %v", err)
			}
		}
	}()

	var (
		err   error
		next  bool
		delay = baseDelay
	)

	for i := range retry {
		if delay < 0 {
			delay = 0
		} else if delay > maxDelay {
			delay = maxDelay
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			if next, err = fn(ctx, i+1); !next {
				return err
			}
		}

		if delay < maxDelay {
			if delay > maxDelay/2 {
				delay = maxDelay
			} else {
				delay *= 2
			}
		}
	}

	return err
}

// GoWithTimeout 并发执行多个协程，并阻塞等待所有协程执行完毕或超时
// 与 Go 不同，本函数会阻塞当前协程直到所有 fn 执行完成或达到超时时间
// @param timeout time.Duration 整体执行的超时时间
// @param fns ...func() 待执行的协程函数
func GoWithTimeout(timeout time.Duration, fns ...func()) {
	NewGoroutines().Add(fns...).Run(context.Background(), timeout)
}

// GoWithDeadline 并发执行多个协程，并阻塞等待所有协程执行完毕或到达最后期限
// 与 Go 不同，本函数会阻塞当前协程直到所有 fn 执行完成或到达最后期限
// @param deadline time.Time 最后期限，到达后停止等待
// @param fns ...func() 待执行的协程函数
func GoWithDeadline(deadline time.Time, fns ...func()) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	NewGoroutines().Add(fns...).Run(ctx)
}
