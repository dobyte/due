package redis

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
	"github.com/redis/go-redis/v9"
)

// Limiter is a distributed token bucket limiter for one scene.
//
// It is stateless with respect to dimension keys: the same Limiter can be shared by concurrent
// callers and different keys.
type Limiter struct {
	maker         *Maker
	client        redis.UniversalClient
	script        *redis.Script
	keyPrefix     string
	sceneKey      string
	rateNanoPerMs int64
	burst         int
	burstNano     int64
	ttl           int64 // Key expiration in milliseconds
	timeout       time.Duration
	failurePolicy limiter.FailurePolicy
}

var _ limiter.Limiter = (*Limiter)(nil)

// Allow takes n tokens from the bucket identified by key and reports the decision. n defaults to
// 1. A denied request is not an error: it is reported by Result.Allowed.
func (l *Limiter) Allow(ctx context.Context, key string, n ...int) (limiter.Result, error) {
	if l.maker.closed.Load() {
		return limiter.Result{}, errors.ErrIllegalOperation
	}

	if l.maker.open.Load() {
		return l.failure(errors.ErrLimiterUnavailable)
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

	ctx, cancel := l.commandContext(ctx)

	reply, err := l.script.Run(
		ctx,
		l.client,
		[]string{bucketKey},
		l.rateNanoPerMs,
		l.burstNano,
		int64(cost)*tokenScale,
		l.ttl,
	).Int64Slice()

	if cancel != nil {
		cancel()
	}

	if err != nil {
		l.maker.recordFailure()
		return l.failure(err)
	}

	if len(reply) != 4 {
		l.maker.recordFailure()
		return l.failure(errors.ErrInvalidFormat)
	}

	l.maker.recordSuccess()

	return limiter.Result{
		Allowed:    reply[0] == 1,
		Limit:      l.burst,
		Remaining:  int(reply[1] / tokenScale),
		RetryAfter: time.Duration(reply[2]) * time.Millisecond,
		ResetAfter: time.Duration(reply[3]) * time.Millisecond,
	}, nil
}

// commandContext applies the configured decision timeout.
//
// When the caller's context already has a shorter deadline, it is returned unchanged so that no
// context or timer is allocated for this call.
func (l *Limiter) commandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if l.timeout <= 0 {
		return ctx, nil
	}

	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= l.timeout {
		return ctx, nil
	}

	return context.WithTimeout(ctx, l.timeout)
}

// failure applies the configured failure policy.
func (l *Limiter) failure(err error) (limiter.Result, error) {
	if l.failurePolicy == limiter.FailClosed {
		return limiter.Result{}, err
	}

	l.maker.warn(err)

	return limiter.Result{Allowed: true, Limit: l.burst, Degraded: true}, nil
}
