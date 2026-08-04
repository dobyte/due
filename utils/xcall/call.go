package xcall

import (
	"context"
	"runtime"
	"time"

	"github.com/dobyte/due/v2/log"
)

// Call 安全地调用函数
func Call(fn func()) {
	if fn == nil {
		return
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

	fn()
}

// Go 执行单个协程
func Go(fn func()) {
	go Call(fn)
}

// Backoff 指数退避调用函数
func Backoff(ctx context.Context, fn func(int) (bool, error), retry int, baseDelay, maxDelay time.Duration) error {
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
		err  error
		next bool
	)

	for i := range retry {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(min((1<<i)*baseDelay, maxDelay)):
			if next, err = fn(i + 1); !next {
				return err
			}
		}
	}

	return err
}

// GoWithTimeout 执行多个协程（附带超时时间）
func GoWithTimeout(timeout time.Duration, fns ...func()) {
	NewGoroutines().Add(fns...).Run(context.Background(), timeout)
}

// GoWithDeadline 执行多个协程（附带最后期限）
func GoWithDeadline(deadline time.Time, fns ...func()) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	NewGoroutines().Add(fns...).Run(ctx)
}
