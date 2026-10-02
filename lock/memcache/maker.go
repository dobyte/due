package memcache

import (
	"context"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
)

const (
	// releaseExpiration is the expiration timestamp written when releasing a lock.
	//
	// memcached parses an expiration beyond 30 days as an absolute timestamp (Unix seconds), and
	// clamps it to "expire immediately" when that timestamp is earlier than the server start time,
	// which is how releasing is achieved. A fixed and sufficiently old absolute timestamp (the year
	// 2001) is used here so that any server's start time is later than it; a relative offset to the
	// current time (such as now-1 year) must not be used, because once the server uptime exceeds
	// that offset the timestamp would be treated as a future absolute time and the lock could never
	// be released.
	releaseExpiration = int32(1000000000)

	// maxSwapRetries is the maximum number of CAS conflict retries.
	//
	// Renewal and release of a lock share the "read-verify-CAS" flow, and concurrent operations on
	// the same lock (such as release and an in-flight renewal) may cause a CAS conflict, which is
	// resolved by re-reading and retrying after the conflict.
	maxSwapRetries = 5
)

// Maker is a lock maker.
//
// It creates distributed locks from the memcached Add/Get/CompareAndSwap primitives. It holds the
// memcached client and the global lock configuration internally, and generates an independent
// version identifier for each lock to verify lock ownership.
type Maker struct {
	opts    *options        // Lock configuration
	builtin bool            // Whether the client is built in; a built-in client is closed along with Close
	ctx     context.Context // Maker lifecycle context, canceled by Close to stop every renewal goroutine
	cancel  context.CancelFunc
}

// NewMaker creates a lock maker.
//
// It initializes the lock configuration: when the expiration is not explicitly configured the
// default value is used, and an expiration below 1 second is clamped to 1 second so that memcached
// does not truncate a sub-second expiration to 0 (never expires). An acquisition interval of 0 or
// less is clamped to the default so that the retry loop does not degenerate into a busy-wait
// without backoff. When no external client is provided, a built-in client is created automatically
// and is closed along with Close.
func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.expiration <= 0 {
		o.expiration = xconv.Duration(defaultExpiration)
	}

	// memcached expiration has 1-second granularity and 0 means never expires;
	// clamp the expiration lower bound to 1s so a sub-second value is not truncated to 0 and the
	// lock never expires.
	if o.expiration < time.Second {
		o.expiration = time.Second
	}

	// The acquisition interval must be greater than 0; 0 or a negative value makes the retry timer
	// fire immediately and degenerates into a busy-wait without backoff.
	if o.acquireInterval <= 0 {
		o.acquireInterval = xconv.Duration(defaultAcquireInterval)
	}

	m := &Maker{}
	m.opts = o
	m.ctx, m.cancel = context.WithCancel(context.Background())

	if o.client == nil {
		o.client = memcache.New(o.addrs...)
		m.builtin = true
	}

	return m
}

// Make makes a Locker.
//
// It builds the memcached key by concatenating the configured prefix with the lock name, and
// generates a unique version identifier for the lock. Every Locker holds an independent version
// identifier so that they can fairly compete for the same lock.
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
// It first cancels the maker lifecycle context to stop every background renewal goroutine, then
// closes the built-in client. When an external client is used, its lifecycle is managed by the
// external caller and is left untouched here.
func (m *Maker) Close() error {
	// Stop every background renewal goroutine; CancelFunc is idempotent and may be called
	// repeatedly.
	m.cancel()

	if m.builtin {
		return m.opts.client.Close()
	}

	return nil
}

// acquire acquires the lock in a loop.
//
// It periodically tries to write the lock with the Add primitive, and a successful write means the
// acquisition succeeded. A failed write (ErrNotStored) means the lock is already held by someone
// else, so it retries at the configured interval up to the maximum number of retries until it
// succeeds, the retries are exhausted, or the ctx is canceled. The ctx is the context whose
// cancellation terminates the acquisition immediately; key is the memcached key and version is the
// lock version identifier. It returns nil on success, [errors.ErrDeadlineExceeded] when the retries
// are exhausted, ctx.Err() when the context is canceled, or [errors.ErrIllegalOperation] when the
// maker has been closed.
func (m *Maker) acquire(ctx context.Context, key, version string) error {
	// Fail fast when the maker has been closed to avoid a successful acquisition whose background
	// renewal silently stops because the lifecycle context was canceled.
	if m.ctx.Err() != nil {
		return errors.ErrIllegalOperation
	}

	var (
		err     error
		retries int
		item    = &memcache.Item{
			Key:        key,
			Value:      xconv.Bytes(version),
			Expiration: expirationSeconds(m.opts.expiration),
		}
	)

	for {
		if err = m.opts.client.Add(item); err == nil {
			return nil
		}

		if !errors.Is(err, memcache.ErrNotStored) {
			return err
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
			// The maker was closed while waiting, stop the acquisition.
			ticker.Stop()
			return errors.ErrIllegalOperation
		case <-ticker.C:
			ticker.Stop()
		}
	}
}

// tryAcquire attempts to acquire the lock.
//
// It performs a single Add write without waiting or retrying. A fixed expiration may be given
// through expiration; when it is not given, the configured default expiration is used. The key is
// the memcached key and version is the lock version identifier. The optional expiration is a fixed
// expiration time; when it is empty or not positive the default expiration is used. It returns nil
// on success, [errors.ErrIllegalOperation] when the lock is already held by someone else, or
// [errors.ErrIllegalOperation] when the maker has been closed.
func (m *Maker) tryAcquire(_ context.Context, key, version string, expiration ...time.Duration) error {
	// Fail fast when the maker has been closed to avoid a successful acquisition whose background
	// renewal silently stops because the lifecycle context was canceled.
	if m.ctx.Err() != nil {
		return errors.ErrIllegalOperation
	}

	item := &memcache.Item{Key: key, Value: xconv.Bytes(version)}

	if len(expiration) > 0 && expiration[0] > 0 {
		item.Expiration = expirationSeconds(expiration[0])
	} else {
		item.Expiration = expirationSeconds(m.opts.expiration)
	}

	if err := m.opts.client.Add(item); err != nil {
		if errors.Is(err, memcache.ErrNotStored) {
			return errors.ErrIllegalOperation
		}

		return err
	}

	return nil
}

// release releases the lock.
//
// It reuses the "read-verify-CAS" flow and rewrites the expiration to a fixed and sufficiently old
// absolute timestamp (see [releaseExpiration]) so that memcached immediately considers the key
// expired and deletes it, which achieves the release. The ctx is the context to use, key is the
// memcached key and version is the lock version identifier. It returns nil on success,
// [errors.ErrIllegalOperation] when the lock has been lost or its ownership has changed, or the
// driver error on a persistent CAS conflict (a transient race).
func (m *Maker) release(ctx context.Context, key, version string) error {
	return m.swap(ctx, key, version, releaseExpiration)
}

// renewal renews the lock.
//
// It refreshes the lock expiration to the configured duration through "read-verify-CAS". When the
// first operation fails (a transient failure) it retries with exponential backoff, keeping the
// total backoff sleep budget within half of the lock expiration (see [backoffRetries]) to avoid the
// backoff blocking long enough for the lock to expire. If the backoff still fails, the renewal
// scheduler (locker.go) keeps compensating retries at the short interval [renewalRetryInterval].
// Once the lock ownership is lost (renewal returns [errors.ErrIllegalOperation]) it terminates
// immediately. The ctx is the context to use, key is the memcached key and version is the lock
// version identifier. It returns nil on success or [errors.ErrIllegalOperation] when the lock has
// been lost or its ownership has changed.
func (m *Maker) renewal(ctx context.Context, key, version string) error {
	var (
		expiration = expirationSeconds(m.opts.expiration)
		renew      = func(ctx context.Context) error {
			return m.swap(ctx, key, version, expiration)
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
		// The expiration is too short and the backoff budget is insufficient, so do not retry with
		// backoff; let the renewal scheduler compensate with retries at the short interval
		// [renewalRetryInterval].
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

// backoffRetries computes the number of renewal backoff retries and the initial interval.
//
// The total duration of the exponential backoff (the interval doubles starting from 100ms) is
// limited to half of the lock expiration, so that the backoff does not block the renewal schedule
// and let the lock expire before the failure recovers. It backs off at most 3 times and does not
// retry when the budget cannot support a single backoff. The expiration is the lock expiration
// duration. It returns the number of backoff retries, at least 0 (no retry when the budget cannot
// support a single backoff), and the initial backoff interval, always 100ms.
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

// expirationSeconds converts an expiration duration to memcached expiration seconds.
//
// memcached expiration has 1-second granularity and 0 means never expires. To avoid a sub-second
// duration being truncated to 0 (never expires), it always rounds up and guarantees a minimum of
// 1 second. The expiration is the lock expiration duration and the returned value is the memcached
// expiration in seconds.
func expirationSeconds(expiration time.Duration) int32 {
	if expiration <= 0 {
		return 0
	}

	return int32(max(int64(1), (expiration.Milliseconds()+999)/1000))
}

// swap performs the replace operation.
//
// The flow is "read-verify-CAS"; concurrent renewal/release operations on the same lock may cause a
// CAS conflict, in which case it re-reads and retries. Definite cases such as a missing lock or a
// changed ownership (a version mismatch) are mapped to [errors.ErrIllegalOperation], while other
// unexpected results are returned as is. When all retries are exhausted due to CAS conflicts it
// returns the conflict error (a transient race) instead of falsely reporting a lost ownership. The
// ctx is the context to use, key is the memcached key, version is the lock version identifier and
// expiration is the new expiration. It returns nil on success, [errors.ErrIllegalOperation] when
// the lock does not exist or its ownership has changed, or memcache.ErrCASConflict when every retry
// hits a CAS conflict.
func (m *Maker) swap(_ context.Context, key, version string, expiration int32) error {
	var casErr error

	for range maxSwapRetries {
		item, err := m.opts.client.Get(key)
		if err != nil {
			if errors.Is(err, memcache.ErrCacheMiss) {
				// The lock does not exist, which means it has expired or was released.
				return errors.ErrIllegalOperation
			}

			return err
		}

		// The lock has been acquired by another holder (a different version).
		if xconv.String(item.Value) != version {
			return errors.ErrIllegalOperation
		}

		item.Expiration = expiration

		if err = m.opts.client.CompareAndSwap(item); err == nil {
			return nil
		}

		switch {
		case errors.Is(err, memcache.ErrCASConflict):
			// Racing with a renewal/release of the same lock; re-read and retry, and record the
			// conflict error.
			casErr = err
		case errors.Is(err, memcache.ErrNotStored), errors.Is(err, memcache.ErrCacheMiss):
			// The lock was deleted or expired between the read and the swap, so its ownership is
			// lost.
			return errors.ErrIllegalOperation
		default:
			return err
		}
	}

	// All retries are exhausted due to CAS conflicts: ownership is not necessarily lost, so return
	// the conflict error as is instead of [errors.ErrIllegalOperation], letting the renewal
	// scheduler treat it as a transient failure and compensate with retries rather than wrongly
	// concluding that the lock is lost and stopping the renewal.
	return casErr
}
