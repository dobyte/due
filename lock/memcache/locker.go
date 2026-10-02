package memcache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// renewalRetryInterval is the compensation retry interval used after a transient failure while
// renewing a lock.
//
// During a failure the renewal keeps retrying at this short interval, instead of waiting for a full
// renewal period, so that a service recovering before the lock expires can renew the lock as soon
// as possible. It also acts as the busy-wait guard interval when the expiration is too short for
// the backoff budget.
const renewalRetryInterval = 100 * time.Millisecond

// Locker is a distributed lock.
//
// It is a renewable distributed lock built on memcached: each lock maps to one memcached key and
// its ownership is defined by an independent version identifier. While the lock is held its
// expiration is refreshed through a "read-verify-CAS" loop, and the lock becomes invalid once it
// expires or its ownership is lost.
type Locker struct {
	maker   *Maker
	key     string
	version string
	cancel  atomic.Value // Holds the renewal cancel function context.CancelFunc; holds a typed nil when no renewal is running
}

// Acquire acquires the lock.
//
// Acquire blocks until the lock is acquired, after which it starts a background renewal to keep
// holding the lock until it is explicitly released or lost. The ctx is the context whose
// cancellation aborts the acquisition. It returns nil on success,
// [errors.ErrDeadlineExceeded] when the retries are exhausted, or ctx.Err() when the context is
// canceled.
func (l *Locker) Acquire(ctx context.Context) error {
	if err := l.maker.acquire(ctx, l.key, l.version); err != nil {
		return err
	}

	l.renewal()

	return nil
}

// TryAcquire attempts to acquire the lock.
//
// TryAcquire makes a single attempt and returns immediately on failure without blocking. On
// success, a positive expiration makes the lock expire once after that duration without renewal;
// otherwise the lock uses the default expiration and starts a background renewal. The ctx is the
// context to use. The optional expiration is a fixed expiration time; when it is empty or not
// positive, the lock is not time-bounded and renewal is enabled. It returns nil on success or
// [errors.ErrIllegalOperation] when the lock is already held by someone else.
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
// Release stops the background renewal first and then releases the lock by version identifier; only
// the owner of the lock (a matching version identifier) can release it successfully. The ctx is the
// context to use. It returns nil on success, [errors.ErrIllegalOperation] when the lock has been
// lost or its ownership has changed, or the driver error on a persistent CAS conflict (a transient
// race).
func (l *Locker) Release(ctx context.Context) error {
	// Atomically swap out the renewal cancel function and stop renewing;
	// when no renewal is running (for example TryAcquire was given a fixed expiration),
	// the swapped-out value is a typed nil.
	if prev := l.cancel.Swap(context.CancelFunc(nil)).(context.CancelFunc); prev != nil {
		prev()
	}

	return l.maker.release(ctx, l.key, l.version)
}

// renewal renews the lock.
//
// It starts a background goroutine that renews the lock periodically: normally once every half of
// the lock expiration. On a transient failure, a single renewal first performs exponential backoff
// retries (see [Maker.renewal]); once the backoff is exhausted it falls back to the short interval
// [renewalRetryInterval] and renews again, so that a recovered service renews as soon as possible
// instead of idly waiting a full period and wasting the recovery window. The goroutine exits when
// the lock is lost (renewal returns [errors.ErrIllegalOperation]) or the renewal is canceled
// (Release/Close). Each call cancels the previous renewal cancel function first, ensuring at most
// one renewal goroutine exists for a Locker at a time. The goroutine is started through [xcall.Go],
// so a panic during execution is recovered automatically and does not bring down the process.
func (l *Locker) renewal() {
	// The renewal ctx is derived from the Maker's context, so Maker.Close cancels every renewal
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
			case err == nil: // Renewal succeeded, restore the normal renewal period
				warned = false
				delay = interval
			case ctx.Err() != nil: // Release/Close canceled the renewal (normal exit path), no warning needed
				return
			case errors.Is(err, errors.ErrIllegalOperation): // The lock has expired or its ownership changed, renewing further is pointless
				log.Warnf("renew lock failed, the lock has been lost: %v", err)
				return
			default: // Transient failure: shorten the wait interval and keep compensating retries to renew before the lock expires; warn only once and restore the normal period after a successful renewal
				if !warned {
					log.Warnf("renew lock failed, will retry in a short interval: %v", err)
					warned = true
				}

				delay = renewalRetryInterval
			}
		}
	})
}
