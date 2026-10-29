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
type limiterBucket struct {
	limiter    *corelimiter.Limiter
	violations atomic.Int32
	closing    atomic.Bool
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

	_, retryAfter, ok := bucket.limiter.TakeAt(time.Now(), 1)
	if ok {
		bucket.violations.Store(0)
		return true, false
	}

	violations := bucket.violations.Add(1)
	if r.closeThreshold > 0 && int(violations) >= r.closeThreshold {
		closeConn = bucket.closing.CompareAndSwap(false, true)
	}

	dropped := r.dropped.Add(1)
	r.warn(cid, retryAfter, dropped, violations, closeConn)

	return false, closeConn
}

// warn logs a throttled warning for a dropped message.
func (r *connLimiter) warn(
	cid int64, retryAfter time.Duration, dropped int64, violations int32, closeConn bool,
) {
	now := time.Now().UnixNano()
	last := r.lastLog.Load()

	if now-last < int64(defaultLimiterLogInterval) {
		return
	}

	if !r.lastLog.CompareAndSwap(last, now) {
		return
	}

	log.Warnf("connection rate limit exceeded, cid: %d, retryAfter: %v, violations: %d, "+
		"dropped: %d, close: %t", cid, retryAfter, violations, dropped, closeConn)
}

// loadShard returns the shard of the given connection ID.
func (r *connLimiter) loadShard(cid int64) *limiterShard {
	return &r.shards[int(uint64(cid)&(defaultLimiterShardNum-1))]
}
