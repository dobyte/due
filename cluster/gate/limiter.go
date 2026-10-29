package gate

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	corelimiter "github.com/dobyte/due/v2/core/limiter"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	// defaultLimiterShardNum is the number of rate limiter table shards (a power of two, so that
	// the shard is selected with a bit operation).
	defaultLimiterShardNum = 256

	// defaultLimiterLogInterval is the minimum interval between two rate limit warnings. The
	// warning is throttled so that a flooding connection cannot amplify its own traffic into logs.
	defaultLimiterLogInterval = time.Second
)

// limiterOptions are the connection rate limiting options.
type limiterOptions struct {
	quota          float64       // quota is the number of requests allowed per period.
	period         time.Duration // period is the quota period; it defaults to one second.
	burst          int           // burst is the bucket capacity; it defaults to ceil(quota).
	closeThreshold int           // consecutive limited messages before closing; zero disables.
}

// normalize validates the options and fills in the defaults. It reports whether rate limiting is
// enabled.
func (o *limiterOptions) normalize() bool {
	if o.quota <= 0 || math.IsNaN(o.quota) || math.IsInf(o.quota, 0) {
		return false
	}

	if o.period <= 0 {
		o.period = xconv.Duration(defaultLimiterPeriod)
	}

	if o.burst <= 0 {
		o.burst = int(math.Ceil(o.quota))
	}

	if o.burst < 1 {
		o.burst = 1
	}

	if o.closeThreshold < 0 {
		o.closeThreshold = 0
	}

	return true
}

// limiterShard is a shard of the per-connection rate limiter table.
//
// Shards are selected with the low bits of the connection ID; the struct is padded to a 64-byte
// cache line to avoid false sharing between adjacent shards.
type limiterShard struct {
	rw      sync.RWMutex
	buckets map[int64]*limiterBucket
	_       [32]byte
}

// limiterBucket holds the token bucket and the consecutive limit violations of one connection.
//
// The violation state is only touched by the connection read loop, so it needs no synchronization.
type limiterBucket struct {
	limiter    *corelimiter.Limiter
	violations int32
	closing    bool
}

// connLimiter holds one token bucket per connection.
//
// Only business frames reach the limiter when session security is enabled: handshake frames are
// routed to the security manager, and heartbeat frames are handled by the network server before
// the receive handler, so both bypass the limit.
//
// A connection is served by a single network read loop, so its bucket is only touched by one
// goroutine at a time; the shard lock only protects the bucket table.
type connLimiter struct {
	burst          int
	rate           float64
	closeThreshold int
	shards         [defaultLimiterShardNum]limiterShard
	lastLog        atomic.Int64 // last warning time in Unix nanoseconds
	_              [56]byte     // keeps dropped on a separate cache line
	dropped        atomic.Int64 // number of rate limited messages
}

// newConnLimiter creates a per-connection rate limiter with normalized options.
func newConnLimiter(opts *limiterOptions) *connLimiter {
	r := &connLimiter{
		burst:          opts.burst,
		rate:           opts.quota / opts.period.Seconds(),
		closeThreshold: opts.closeThreshold,
	}

	for i := range r.shards {
		r.shards[i].buckets = make(map[int64]*limiterBucket)
	}

	return r
}

// add creates the bucket of cid when it is absent. The bucket starts full, so a new connection may
// burst up to the configured capacity.
func (r *connLimiter) add(cid int64) {
	shard := r.loadShard(cid)
	shard.rw.Lock()
	if _, ok := shard.buckets[cid]; !ok {
		shard.buckets[cid] = &limiterBucket{
			limiter: corelimiter.NewLimiter(float64(r.burst), r.rate),
		}
	}
	shard.rw.Unlock()
}

// remove releases the bucket of cid.
func (r *connLimiter) remove(cid int64) {
	shard := r.loadShard(cid)
	shard.rw.Lock()
	delete(shard.buckets, cid)
	shard.rw.Unlock()
}

// allow reports whether a message from cid may be processed and whether the connection should be
// closed because it reached the configured consecutive violation threshold. A missing bucket is
// allowed, so a connection registered outside the gate admission path is never dropped silently.
func (r *connLimiter) allow(cid int64) (allowed bool, closeConn bool) {
	shard := r.loadShard(cid)
	shard.rw.RLock()
	bucket := shard.buckets[cid]
	shard.rw.RUnlock()

	if bucket == nil {
		return true, false
	}

	now := time.Now()
	_, retryAfter, ok := bucket.limiter.TakeAt(now, 1)
	if ok {
		if bucket.violations != 0 {
			bucket.violations = 0
		}
		return true, false
	}

	if r.closeThreshold > 0 {
		bucket.violations++
		if int(bucket.violations) >= r.closeThreshold && !bucket.closing {
			bucket.closing = true
			closeConn = true
		}
	}

	dropped := r.dropped.Add(1)
	if closeConn || r.shouldLog(now) {
		r.warn(now, cid, retryAfter, dropped, bucket.violations, closeConn)
	}

	return false, closeConn
}

// shouldLog reports whether a warning may be emitted at now. It keeps the throttled logging path
// out of the hot drop path while the intervals between warnings are short.
func (r *connLimiter) shouldLog(now time.Time) bool {
	return now.UnixNano()-r.lastLog.Load() >= int64(defaultLimiterLogInterval)
}

// warn logs a throttled warning for a dropped message.
func (r *connLimiter) warn(
	now time.Time, cid int64, retryAfter time.Duration, dropped int64, violations int32, closeConn bool,
) {
	nowNano := now.UnixNano()
	last := r.lastLog.Load()

	if nowNano-last < int64(defaultLimiterLogInterval) {
		return
	}

	if !r.lastLog.CompareAndSwap(last, nowNano) {
		return
	}

	log.Warnf("connection rate limit exceeded, cid: %d, retryAfter: %v, violations: %d, "+
		"dropped: %d, close: %t", cid, retryAfter, violations, dropped, closeConn)
}

// loadShard returns the shard of the given connection ID.
func (r *connLimiter) loadShard(cid int64) *limiterShard {
	return &r.shards[int(uint64(cid)&(defaultLimiterShardNum-1))]
}
