package redis

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// renewalRetryInterval is the compensation retry interval used after a transient failure during
// renewal.
//
// During a failure it keeps retrying at this short interval instead of waiting for a full renewal
// period, so that renewal succeeds as soon as the service recovers before the lock expires; it also
// acts as a busy-wait protection interval when the expiration is too short and the backoff budget
// is insufficient.
const renewalRetryInterval = 100 * time.Millisecond

// Locker is a distributed lock.
//
// It is a renewable distributed lock built on redis: each lock maps to one redis key, and ownership
// is defined by a separate version identifier. While the lock is held, its expiration is refreshed
// periodically by the renewal script; it becomes invalid once it expires or ownership is lost.
type Locker struct {
	maker   *Maker
	key     string
	version string
	cancel  atomic.Value // Stores the context.CancelFunc that cancels renewal; its value is a typed nil when no renewal is running.
}

// Acquire acquires the lock.
//
// It acquires the lock in blocking mode and, on success, starts a background renewal so that the
// lock keeps being held until it is released explicitly or lost.
//
// It returns nil on success, [errors.ErrDeadlineExceeded] when the retries are exhausted, and the
// error reported by ctx when ctx is canceled.
func (l *Locker) Acquire(ctx context.Context) error {
	if err := l.maker.acquire(ctx, l.key, l.version); err != nil {
		return err
	}

	l.renewal()

	return nil
}

// TryAcquire tries to acquire the lock.
//
// It attempts the acquisition only once and returns immediately without blocking when the attempt
// fails. On success, when a positive expiration is given the lock expires once after that duration
// and is not renewed; otherwise the default expiration applies and a background renewal is started.
//
// It returns nil on success and [errors.ErrIllegalOperation] when the lock is already held by
// someone else.
func (l *Locker) TryAcquire(ctx context.Context, expiration ...time.Duration) error {
	if err := l.maker.tryAcquire(ctx, l.key, l.version, expiration...); err != nil {
		return err
	}

	if len(expiration) == 0 || expiration[0] <= 0 {
		l.renewal()
	}

	return nil
}

// Release releases the lock.
//
// It stops the background renewal first and then releases the lock by version identifier; only the
// lock owner (whose version identifier matches) can release it successfully.
//
// It returns nil on success and [errors.ErrIllegalOperation] when the lock has been lost or its
// ownership has changed.
func (l *Locker) Release(ctx context.Context) error {
	// Atomically swap out the renewal cancel function and stop renewal. When no renewal is running
	// (for example, when TryAcquire was given a fixed expiration), the swapped-out value is a typed
	// nil.
	if prev := l.cancel.Swap(context.CancelFunc(nil)).(context.CancelFunc); prev != nil {
		prev()
	}

	return l.maker.release(ctx, l.key, l.version)
}

// renewal renews the lock.
//
// It starts a background goroutine that renews the lock periodically: under normal conditions it
// renews once every half of the lock expiration. When a transient failure occurs during renewal, a
// single renewal first performs exponential backoff retries (see [Maker.renewal]); once the backoff
// is exhausted, renewal proceeds again at a shorter interval ([renewalRetryInterval]), so that it
// succeeds as soon as the failure is recovered instead of waiting out a full period and wasting the
// recovery window. The goroutine exits when the lock is lost (reported as
// [errors.ErrIllegalOperation]) or when renewal is canceled (Release/Close).
//
// Every call first cancels the previous renewal cancel function, ensuring that at most one renewal
// goroutine exists for a Locker at a time. The goroutine is started through [xcall.Go], so a panic
// raised during its execution is recovered automatically and does not bring down the process.
func (l *Locker) renewal() {
	// The renewal context is derived from the Maker context, so Maker.Close cancels every renewal
	// goroutine at once.
	ctx, cancel := context.WithCancel(l.maker.ctx)

	if prev := l.cancel.Swap(cancel).(context.CancelFunc); prev != nil {
		prev()
	}

	xcall.Go(func() {
		var (
			interval = l.maker.opts.expiration / 2 // Normal renewal period
			delay    = interval                    // Wait time until the next renewal
			warned   bool
		)

		for {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			switch err := l.maker.renewal(ctx, l.key, l.version); {
			case err == nil: // Renewal succeeded, restore the normal renewal period.
				warned = false
				delay = interval
			case ctx.Err() != nil: // Release/Close has already canceled renewal (the normal exit path), no warning is needed.
				return
			case errors.Is(err, errors.ErrIllegalOperation): // The lock has expired or its ownership has changed, renewing further is pointless.
				log.Warnf("renew lock failed, the lock has been lost: %v", err)
				return
			default: // Transient failure: shorten the wait interval and keep compensating; renew as soon as possible before the lock expires; warn only once and restore the normal period after a successful renewal.
				if !warned {
					log.Warnf("renew lock failed, will retry in a short interval: %v", err)
					warned = true
				}

				delay = renewalRetryInterval
			}
		}
	})
}
