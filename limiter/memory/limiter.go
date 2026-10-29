package memory

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
)

// Limiter is an in-process token bucket limiter for one scene.
//
// It is stateless with respect to dimension keys: the same Limiter can be shared by concurrent
// callers and different keys.
type Limiter struct {
	maker     *Maker
	keyPrefix string
	sceneKey  string
	rate      float64
	burst     int
	ttl       time.Duration
}

var _ limiter.Limiter = (*Limiter)(nil)

// Allow takes n tokens from the bucket identified by key and reports the decision. n defaults to
// 1. A denied request is not an error: it is reported by Result.Allowed.
func (l *Limiter) Allow(_ context.Context, key string, n ...int) (limiter.Result, error) {
	if l.maker.closed.Load() {
		return limiter.Result{}, errors.ErrIllegalOperation
	}

	cost := 1
	if len(n) > 0 && n[0] > 0 {
		cost = n[0]
	}

	if cost > l.burst {
		return limiter.Result{}, errors.ErrInvalidLimiterCost
	}

	bucketKey := l.sceneKey
	if key != "" {
		bucketKey = l.keyPrefix + key
	}

	b := l.maker.bucket(bucketKey, l)
	remaining, retryAfter, allowed := b.limiter.Take(cost)
	b.lastSeen.Store(time.Now().UnixNano())

	return limiter.Result{
		Allowed:    allowed,
		Limit:      l.burst,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAfter: l.resetAfter(remaining),
	}, nil
}

// resetAfter returns the time needed to refill the bucket from remaining to full.
func (l *Limiter) resetAfter(remaining int) time.Duration {
	if l.rate <= 0 || remaining >= l.burst {
		return 0
	}

	resetAfter := time.Duration(float64(l.burst-remaining) / l.rate * float64(time.Second))
	if resetAfter < time.Millisecond {
		resetAfter = time.Millisecond
	}

	return resetAfter
}
