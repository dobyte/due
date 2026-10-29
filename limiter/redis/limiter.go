package redis

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
)

// Limiter is a distributed token bucket limiter for one scene.
//
// It is stateless with respect to dimension keys: the same Limiter can be shared by concurrent
// callers and different keys.
type Limiter struct {
	maker     *Maker
	keyPrefix string
	sceneKey  string
	rate      float64
	burst     int
	ttl       int64 // Key expiration in milliseconds
}

var _ limiter.Limiter = (*Limiter)(nil)

// Allow takes n tokens from the bucket identified by key and reports the decision. n defaults to
// 1. A denied request is not an error: it is reported by Result.Allowed.
func (l *Limiter) Allow(ctx context.Context, key string, n ...int) (limiter.Result, error) {
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

	var cancel context.CancelFunc
	if l.maker.opts.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, l.maker.opts.timeout)
	}

	reply, err := l.maker.script.Run(
		ctx,
		l.maker.opts.client,
		[]string{bucketKey},
		time.Now().UnixMilli(),
		l.rate,
		l.burst,
		cost,
		l.ttl,
	).Int64Slice()

	if cancel != nil {
		cancel()
	}

	if err != nil {
		return l.failure(err)
	}

	if len(reply) != 4 {
		return l.failure(errors.ErrInvalidFormat)
	}

	return limiter.Result{
		Allowed:    reply[0] == 1,
		Limit:      l.burst,
		Remaining:  int(reply[1]),
		RetryAfter: time.Duration(reply[2]) * time.Millisecond,
		ResetAfter: time.Duration(reply[3]) * time.Millisecond,
	}, nil
}

// failure applies the configured failure policy.
func (l *Limiter) failure(err error) (limiter.Result, error) {
	if l.maker.opts.failurePolicy == limiter.FailClosed {
		return limiter.Result{}, err
	}

	l.maker.warn(err)

	return limiter.Result{Allowed: true, Limit: l.burst, Degraded: true}, nil
}
