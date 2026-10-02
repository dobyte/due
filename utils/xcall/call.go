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

// Call invokes fn safely.
//
// It recovers from any panic raised while fn runs: a runtime error ([runtime.Error]) is logged as
// a fatal error, while any other panic is logged together with its error message.
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

// Go runs fn in a new goroutine.
//
// fn is executed in its own goroutine and any panic is recovered by [Call], so a crashing
// goroutine does not bring down the whole process.
func Go[T funcType](fn T) {
	go Call(fn)
}

// Backoff calls fn with an exponentially increasing delay.
//
// The delay starts at baseDelay, doubles after every attempt and is capped at maxDelay. Retrying
// stops when fn returns next as false, when ctx is cancelled, or after retry attempts have been
// made. The attempt argument passed to fn starts at 1.
//
// It returns the error of the last fn call, or ctx.Err() when ctx has been cancelled.
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

// GoWithTimeout runs fns concurrently and blocks until all of them finish or the timeout elapses.
//
// Unlike [Go], it blocks the calling goroutine until every fn has completed or the timeout is
// reached.
func GoWithTimeout(timeout time.Duration, fns ...func()) {
	NewGoroutines().Add(fns...).Run(context.Background(), timeout)
}

// GoWithDeadline runs fns concurrently and blocks until all of them finish or the deadline is
// reached.
//
// Unlike [Go], it blocks the calling goroutine until every fn has completed or the deadline is
// reached, at which point it stops waiting.
func GoWithDeadline(deadline time.Time, fns ...func()) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	NewGoroutines().Add(fns...).Run(ctx)
}
