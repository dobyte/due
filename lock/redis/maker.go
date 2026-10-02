package redis

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/redis/go-redis/v9"
)

// Maker is a lock maker.
//
// It builds distributed locks on top of the redis SET NX primitive and the release/renewal Lua
// scripts. It holds a redis client and the global lock options, and generates a separate version
// identifier for every lock to verify lock ownership.
type Maker struct {
	err           error           // Initialization error (such as a TLS configuration failure); when non-nil every operation returns it directly.
	opts          *options        // Lock options
	builtin       bool            // Reports whether the client is built in; a built-in client is also closed by Close.
	ctx           context.Context // Maker lifecycle context, canceled by Close, used to stop every renewal goroutine.
	cancel        context.CancelFunc
	releaseScript *redis.Script // Lua script that releases a lock
	renewalScript *redis.Script // Lua script that renews a lock
}

// NewMaker creates a lock maker.
//
// It initializes the lock options: when no expiration is configured explicitly the default is used,
// and an expiration shorter than one millisecond is clamped to one millisecond so that a
// sub-millisecond value is not truncated to zero (which would leave the lock without an expiration
// on Set and delete the lock immediately on PEXPIRE 0). When the interval of the acquire loop is
// less than or equal to 0 it is clamped to the default, so that retries do not degenerate into a
// busy-wait loop without backoff. When no external client is provided, a built-in client is created
// and closed by Close.
func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.expiration <= 0 {
		o.expiration = xconv.Duration(defaultExpiration)
	}

	// The redis expiration has a precision of 1 millisecond and 0 means no expiration (whereas
	// PEXPIRE 0 deletes the key immediately); clamp the lower bound of the expiration to 1
	// millisecond so that a sub-millisecond value is not truncated to 0 and makes the lock never
	// expire or expire immediately.
	if o.expiration < time.Millisecond {
		o.expiration = time.Millisecond
	}

	// The acquire interval must be greater than 0; a value of 0 or less makes the retry timer fire
	// immediately and degenerates into a busy-wait loop without backoff.
	if o.acquireInterval <= 0 {
		o.acquireInterval = xconv.Duration(defaultAcquireInterval)
	}

	m := &Maker{}
	m.opts = o
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.releaseScript = redis.NewScript(releaseScript)
	m.renewalScript = redis.NewScript(renewalScript)

	if m.opts.client == nil {
		options := &redis.UniversalOptions{
			Addrs:      m.opts.addrs,
			DB:         m.opts.db,
			Username:   m.opts.username,
			Password:   m.opts.password,
			MaxRetries: m.opts.maxRetries,
		}

		if m.opts.certFile != "" && m.opts.keyFile != "" && m.opts.caFile != "" {
			if options.TLSConfig, m.err = tls.MakeRedisTLSConfig(m.opts.certFile, m.opts.keyFile, m.opts.caFile); m.err != nil {
				return m
			}
		}

		m.opts.client, m.builtin = redis.NewUniversalClient(options), true
	}

	return m
}

// Make creates a locker.
//
// It builds the redis key by joining the configured prefix with the lock name and generates a
// unique version identifier for the lock; every Locker holds its own version identifier so that
// contenders can compete fairly for the same lock.
func (m *Maker) Make(name string) lock.Locker {
	l := &Locker{}
	l.maker = m
	l.version = xuuid.UUID()
	l.cancel.Store(context.CancelFunc(nil))

	if m.opts.prefix == "" {
		l.key = name
	} else {
		l.key = m.opts.prefix + ":" + name
	}

	return l
}

// Close closes the maker.
//
// It first cancels the maker lifecycle context to stop every background renewal goroutine and then
// closes the built-in client. When an external client is used, its lifecycle is managed by the
// caller and is left untouched here.
func (m *Maker) Close() error {
	// Stop every background renewal goroutine; CancelFunc is idempotent and may be called repeatedly.
	m.cancel()

	if m.err != nil {
		return m.err
	}

	if m.builtin {
		return m.opts.client.Close()
	}

	return nil
}

// acquire acquires the lock in a loop.
//
// It periodically tries to write the lock with the SET NX primitive; a successful write means a
// successful acquisition. A failed write (which returns redis.Nil) means the lock is already held
// by someone else, so it retries at the configured interval up to the maximum number of retries,
// until it succeeds, the retries are exhausted, or ctx is canceled.
//
// It returns nil on success, [errors.ErrDeadlineExceeded] when the retries are exhausted, the error
// reported by ctx when ctx is canceled, and [errors.ErrIllegalOperation] when the maker has been
// closed.
func (m *Maker) acquire(ctx context.Context, key, version string) error {
	if m.err != nil {
		return m.err
	}

	// Fail fast when the maker has been closed, so that a successful acquisition is not followed by
	// a background renewal that silently stops because the lifecycle context was canceled.
	if m.ctx.Err() != nil {
		return errors.ErrIllegalOperation
	}

	var (
		args    = redis.SetArgs{Mode: "NX", TTL: m.opts.expiration}
		retries int
	)

	for {
		val, err := m.opts.client.SetArgs(ctx, key, version, args).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}

		if val == "OK" {
			return nil
		}

		if m.opts.acquireMaxRetries > 0 {
			if retries >= m.opts.acquireMaxRetries {
				return errors.ErrDeadlineExceeded
			}

			retries++
		}

		ticker := time.NewTimer(m.opts.acquireInterval)
		select {
		case <-ctx.Done():
			ticker.Stop()
			return ctx.Err()
		case <-m.ctx.Done():
			// The maker was closed while waiting, stop acquiring.
			ticker.Stop()
			return errors.ErrIllegalOperation
		case <-ticker.C:
			ticker.Stop()
		}
	}
}

// tryAcquire tries to acquire the lock.
//
// It performs a single SET NX write without waiting or retrying; a fixed expiration may be given
// through expiration, otherwise the configured default expiration is used.
//
// It returns nil on success, and [errors.ErrIllegalOperation] when the lock is already held by
// someone else or the maker has been closed.
func (m *Maker) tryAcquire(ctx context.Context, key, version string, expiration ...time.Duration) error {
	if m.err != nil {
		return m.err
	}

	// Fail fast when the maker has been closed, so that a successful acquisition is not followed by
	// a background renewal that silently stops because the lifecycle context was canceled.
	if m.ctx.Err() != nil {
		return errors.ErrIllegalOperation
	}

	args := redis.SetArgs{Mode: "NX", TTL: m.opts.expiration}

	if len(expiration) > 0 && expiration[0] > 0 {
		args.TTL = expiration[0]
	}

	val, err := m.opts.client.SetArgs(ctx, key, version, args).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	if val != "OK" {
		return errors.ErrIllegalOperation
	}

	return nil
}

// release releases the lock.
//
// It deletes the lock atomically by version identifier through a Lua script; only the lock owner
// (whose version identifier matches) can release it successfully.
//
// It returns nil on success and [errors.ErrIllegalOperation] when the lock has been lost or its
// ownership has changed.
func (m *Maker) release(ctx context.Context, key, version string) error {
	if m.err != nil {
		return m.err
	}

	rst, err := m.releaseScript.Run(ctx, m.opts.client, []string{key}, version).Int()
	if err != nil {
		return err
	}

	if rst != 1 {
		return errors.ErrIllegalOperation
	}

	return nil
}

// renewal renews the lock.
//
// It refreshes the lock expiration atomically by version identifier through a Lua script. When the
// first attempt fails (a transient failure) it retries with exponential backoff, keeping the total
// backoff sleeping budget within half of the lock expiration (see [backoffRetries]) so that the
// backoff is unlikely to block renewal long enough for the lock to expire. If the backoff still
// fails, the renewal scheduler (locker.go) keeps compensating at a short interval
// ([renewalRetryInterval]). It terminates immediately once lock ownership is lost (reported as
// [errors.ErrIllegalOperation]).
//
// It returns nil on success and [errors.ErrIllegalOperation] when the lock has been lost or its
// ownership has changed.
func (m *Maker) renewal(ctx context.Context, key, version string) error {
	if m.err != nil {
		return m.err
	}

	var (
		keys       = []string{key}
		expiration = m.opts.expiration.Milliseconds()
		renew      = func(ctx context.Context) error {
			rst, err := m.renewalScript.Run(ctx, m.opts.client, keys, version, expiration).Int()
			if err != nil {
				return err
			}

			if rst != 1 {
				return errors.ErrIllegalOperation
			}

			return nil
		}
	)

	err := renew(ctx)
	if err == nil {
		return nil
	}

	if errors.Is(err, errors.ErrIllegalOperation) {
		return err
	}

	retries, baseDelay := backoffRetries(m.opts.expiration)
	if retries == 0 {
		// The expiration is too short and the backoff budget is insufficient, so no backoff retry is
		// performed; the renewal scheduler compensates at the short interval ([renewalRetryInterval]).
		return err
	}

	return xcall.Backoff(ctx, func(ctx context.Context, attempt int) (bool, error) {
		err := renew(ctx)
		if err == nil || errors.Is(err, errors.ErrIllegalOperation) {
			return false, err
		}

		return true, err
	}, retries, baseDelay, time.Second)
}

// backoffRetries computes the number of retries and the initial interval of the renewal backoff.
//
// The total duration of the exponential backoff (the interval doubles starting from 100ms) is
// limited to half of the lock expiration, so that a blocked backoff does not delay the renewal
// schedule and let the lock expire before the failure is recovered. It backs off at most three
// times and does not retry when the budget cannot cover a single backoff.
//
// It returns the number of backoff retries, which is at least 0 (no retry when the budget cannot
// cover a single backoff), and the initial backoff interval, which is always 100ms.
func backoffRetries(expiration time.Duration) (int, time.Duration) {
	var (
		delay   = 100 * time.Millisecond
		budget  = expiration / 2
		retries = 0
	)

	for retries < 3 && delay <= budget {
		budget -= delay
		delay *= 2
		retries++
	}

	return retries, 100 * time.Millisecond
}
