// Package limiter provides a distributed rate limiting abstraction built on a [Maker]/[Limiter]
// pair.
//
// A [Maker] creates [Limiter] instances for named scenes, and a [Limiter] decides whether requests
// identified by a dimension key are allowed. The package holds a global maker so that limiters can
// be created through [Make] without passing the maker around explicitly.
package limiter

import (
	"context"
	"math"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

// FailurePolicy determines how a backend behaves when it cannot make a decision.
type FailurePolicy int

const (
	// FailOpen allows the request when the backend is unavailable.
	FailOpen FailurePolicy = iota + 1
	// FailClosed rejects the request when the backend is unavailable.
	FailClosed
)

// Rule describes a rate limiting rule.
//
// Quota is the number of requests allowed per Period. Period defaults to one second and Burst
// defaults to ceil(Quota), so a zero Burst allows a burst equal to the quota.
type Rule struct {
	Quota  float64       // Quota is the number of requests allowed per Period.
	Period time.Duration // Period is the quota period; it defaults to one second.
	Burst  int           // Burst is the bucket capacity; it defaults to ceil(Quota).
}

// Normalize validates the rule and fills in its defaults.
func (r Rule) Normalize() (Rule, error) {
	if r.Quota <= 0 || math.IsNaN(r.Quota) || math.IsInf(r.Quota, 0) {
		return Rule{}, errors.ErrInvalidLimiterRule
	}

	if r.Period <= 0 {
		r.Period = time.Second
	}

	if r.Burst <= 0 {
		r.Burst = int(math.Ceil(r.Quota))
	}

	if r.Burst < 1 {
		r.Burst = 1
	}

	return r, nil
}

// Rate returns the refill rate in tokens per second.
//
// It must be called on a normalized rule.
func (r Rule) Rate() float64 {
	return r.Quota / r.Period.Seconds()
}

// TTL returns the key expiration for the rule.
//
// The expiration covers the time needed to refill an empty bucket plus one full period, so that an
// idle key expires only after the bucket would have been full again.
func (r Rule) TTL() time.Duration {
	var (
		rate   = r.Rate()
		refill time.Duration
	)

	if rate > 0 {
		refill = time.Duration(math.Ceil(float64(r.Burst) / rate * float64(time.Second)))
	}

	return max(time.Second, max(r.Period, refill)+r.Period)
}

// Result reports the outcome of a rate limiting decision.
type Result struct {
	Allowed    bool          // Allowed reports whether the request was allowed.
	Limit      int           // Limit is the bucket capacity.
	Remaining  int           // Remaining is the number of tokens left after the decision.
	RetryAfter time.Duration // RetryAfter is the suggested wait time when the request is denied.
	ResetAfter time.Duration // ResetAfter is the time needed to refill the bucket.
	Degraded   bool          // Degraded reports whether the request was allowed by fail-open.
}

// Limiter decides whether requests identified by key are allowed for a scene.
type Limiter interface {
	// Allow takes n tokens from the bucket identified by key and reports the decision. n defaults
	// to 1. A denied request is not an error: it is reported by Result.Allowed.
	Allow(ctx context.Context, key string, n ...int) (Result, error)
}

// Maker creates and closes limiters.
type Maker interface {
	// Make creates a limiter for the named scene and rule.
	Make(name string, rule Rule) (Limiter, error)
	// Close closes the maker.
	Close() error
}

var globalMaker Maker

// SetMaker sets the global limiter maker. It closes the previous maker, if any, before replacing
// it.
func SetMaker(maker Maker) {
	if maker == nil {
		log.Warn("cannot set a nil limiter-maker")
		return
	}

	if globalMaker != nil {
		if err := globalMaker.Close(); err != nil {
			log.Error("close limiter-maker failed: %v", err)
		}
	}

	globalMaker = maker
}

// GetMaker returns the global limiter maker.
func GetMaker() Maker {
	return globalMaker
}

// Make creates a limiter for the named scene and rule through the global maker. It returns
// [errors.ErrMissingLimiterInstance] when no maker has been set.
func Make(name string, rule Rule) (Limiter, error) {
	if globalMaker == nil {
		return nil, errors.ErrMissingLimiterInstance
	}

	return globalMaker.Make(name, rule)
}

// Close closes the global maker. It does nothing when no maker has been set.
func Close() error {
	if globalMaker == nil {
		return nil
	}

	return globalMaker.Close()
}
