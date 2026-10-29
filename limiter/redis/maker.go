// Package redis provides a distributed limiter backend on top of redis.
//
// Each rate limiting key maps to one redis hash holding the token bucket state, and every
// decision is made by one Lua script so that refilling and taking tokens are atomic. The backend
// uses the redis server time when available, so clock skew between application nodes does not
// affect the refill.
package redis

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
	"github.com/dobyte/due/v2/log"
	"github.com/redis/go-redis/v9"
)

const (
	// warnInterval is the minimum interval between two backend failure warnings.
	warnInterval = 5 * time.Second

	// circuitBreakerThreshold is the number of consecutive backend failures that opens the circuit.
	circuitBreakerThreshold = 5

	// circuitBreakerCooldown is how long the circuit stays open before a probe is allowed.
	circuitBreakerCooldown = time.Second
)

// Maker is a redis limiter maker.
type Maker struct {
	err      error         // Initialization error, such as a TLS configuration failure.
	opts     *options      // Limiter options
	builtin  bool          // Reports whether the client is built in.
	script   *redis.Script // Lua script that makes a decision.
	closed   atomic.Bool   // Reports whether the maker has been closed.
	lastWarn atomic.Int64  // Unix nanoseconds of the last backend failure warning.
	failures atomic.Int32  // Consecutive backend failures.
	open     atomic.Bool   // Reports whether the circuit is open.

	openDuration time.Duration // How long the circuit stays open.
}

var _ limiter.Maker = (*Maker)(nil)

// NewMaker creates a redis limiter maker.
func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.timeout < 0 {
		o.timeout = 0
	}

	if o.failurePolicy != limiter.FailClosed {
		o.failurePolicy = limiter.FailOpen
	}

	m := &Maker{
		opts:         o,
		script:       redis.NewScript(allowScript),
		openDuration: circuitBreakerCooldown,
	}

	if o.client == nil {
		options := &redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		}

		if o.certFile != "" && o.keyFile != "" && o.caFile != "" {
			if options.TLSConfig, m.err = tls.MakeRedisTLSConfig(o.certFile, o.keyFile, o.caFile); m.err != nil {
				return m
			}
		}

		o.client, m.builtin = redis.NewUniversalClient(options), true
	}

	return m
}

// Make creates a limiter for the named scene and rule.
//
// The name identifies the scene and must be stable for the lifetime of the maker; reusing the
// same name with a different rule shares the existing bucket state.
func (m *Maker) Make(name string, rule limiter.Rule) (limiter.Limiter, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.closed.Load() {
		return nil, errors.ErrIllegalOperation
	}

	if name == "" {
		return nil, errors.ErrInvalidLimiterRule
	}

	normalized, err := rule.Normalize()
	if err != nil {
		return nil, err
	}

	rate := normalized.Rate()
	if int64(normalized.Burst) > maxExactLuaInteger/tokenScale || rate > float64(maxExactLuaInteger)/1e6 {
		return nil, errors.ErrInvalidLimiterRule
	}

	l := &Limiter{
		maker:         m,
		client:        m.opts.client,
		script:        m.script,
		rateNanoPerMs: rateToNanoPerMs(rate),
		burst:         normalized.Burst,
		burstNano:     int64(normalized.Burst) * tokenScale,
		ttl:           max(1, normalized.TTL().Milliseconds()),
		timeout:       m.opts.timeout,
		failurePolicy: m.opts.failurePolicy,
	}

	if m.opts.prefix == "" {
		l.keyPrefix, l.sceneKey = name+":", name
	} else {
		l.keyPrefix = m.opts.prefix + ":" + name + ":"
		l.sceneKey = m.opts.prefix + ":" + name
	}

	return l, nil
}

// rateToNanoPerMs converts a token-per-second rate into nano-tokens per millisecond.
//
// Very low rates are rounded to the nearest nano-token per millisecond; a positive rate never
// becomes zero.
func rateToNanoPerMs(rate float64) int64 {
	n := math.Round(rate * 1e6)
	if n < 1 {
		return 1
	}
	if n > float64(maxExactLuaInteger) {
		return maxExactLuaInteger
	}

	return int64(n)
}

// recordSuccess resets the consecutive backend failure count.
func (m *Maker) recordSuccess() {
	m.failures.Store(0)
}

// recordFailure counts a backend failure and opens the circuit after the threshold.
func (m *Maker) recordFailure() {
	if m.failures.Add(1) < circuitBreakerThreshold {
		return
	}

	if m.open.CompareAndSwap(false, true) {
		time.AfterFunc(m.openDuration, func() {
			m.failures.Store(0)
			m.open.Store(false)
		})
	}
}

// Close closes the maker and the built-in client.
func (m *Maker) Close() error {
	if m.closed.Swap(true) {
		return nil
	}

	if m.err != nil {
		return m.err
	}

	if m.builtin {
		return m.opts.client.Close()
	}

	return nil
}

// warn logs a backend failure at most once per warnInterval.
func (m *Maker) warn(err error) {
	now := time.Now().UnixNano()
	last := m.lastWarn.Load()
	if now-last < int64(warnInterval) || !m.lastWarn.CompareAndSwap(last, now) {
		return
	}

	log.Warnf("rate limiter backend failed: %v", err)
}
