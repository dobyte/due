package gate

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

// Limiter defaults.
const (
	// defaultLimiterShardNum is the number of rate limiter table shards (a power of two, so that
	// the shard is selected with a bit operation).
	defaultLimiterShardNum = 256

	// defaultLimiterReportInterval is the interval between two aggregated drop reports.
	defaultLimiterReportInterval = time.Second

	// defaultLimiterClockInterval is the update interval of the cached limiter clock.
	defaultLimiterClockInterval = time.Millisecond

	// defaultLimiterCoarseFactor is the minimum number of clock intervals per emission interval
	// required to use the cached clock.
	defaultLimiterCoarseFactor = 10

	// defaultLimiterFreeListSize is the maximum number of free buckets retained per shard.
	defaultLimiterFreeListSize = 1024
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
// Shards are selected with the low bits of the connection ID. The sync.Map, the drop counter and
// the free list are kept on separate cache lines so that their updates do not contend.
type limiterShard struct {
	buckets   sync.Map       // per-connection GCRA buckets
	_         [16]byte       // keeps dropped on a separate cache line
	dropped   atomic.Int64   // number of messages dropped by the shard
	_         [56]byte       // keeps the free list on a separate cache line
	freeMu    sync.Mutex     // protects the free list
	free      *limiterBucket // free buckets available for reuse
	freeCount int            // number of buckets in the free list
	_         [40]byte
}

// limiterBucket holds the GCRA state and the consecutive limit violations of one connection.
//
// The state is only touched by the connection read loop, so it needs no synchronization. The
// padding keeps each bucket on its own cache line. next links buckets in the shard free list and is
// only used while the bucket is not in use.
type limiterBucket struct {
	tat        int64          // next allowed time, relative to the limiter epoch
	violations int32          // consecutive limited messages
	closing    bool           // whether the connection close has been signaled
	next       *limiterBucket // free-list link while the bucket is free
	_          [40]byte
}

// connLimiter holds one GCRA bucket per connection.
//
// Only business frames reach the limiter when session security is enabled: handshake frames are
// routed to the security manager, and heartbeat frames are handled by the network server before
// the receive handler, so both bypass the limit.
//
// A connection is served by a single network read loop, so its bucket is only touched by one
// goroutine at a time; the shard lock only protects the bucket table.
type connLimiter struct {
	epoch          time.Time                            // reference time for GCRA timestamps
	interval       int64                                // emission interval in nanoseconds
	tolerance      int64                                // burst tolerance in nanoseconds
	closeThreshold int                                  // consecutive violations before closing; zero disables
	coarse         bool                                 // whether the cached clock is used
	_              [15]byte                             // aligns clock to a cache line
	clock          atomic.Int64                         // cached time since epoch in nanoseconds
	_              [56]byte                             // keeps shards off the clock line
	shards         [defaultLimiterShardNum]limiterShard // per-connection bucket table
	wg             sync.WaitGroup                       // background reporter and clock goroutines
}

// newConnLimiter creates a per-connection rate limiter with normalized options.
func newConnLimiter(opts *limiterOptions) *connLimiter {
	interval := limiterInterval(opts)

	r := &connLimiter{
		epoch:          time.Now(),
		interval:       interval,
		tolerance:      limiterTolerance(interval, opts.burst),
		closeThreshold: opts.closeThreshold,
		coarse:         interval >= int64(defaultLimiterClockInterval)*defaultLimiterCoarseFactor,
	}

	return r
}

// limiterInterval returns the GCRA emission interval in nanoseconds.
func limiterInterval(opts *limiterOptions) int64 {
	nanos := float64(opts.period) / opts.quota

	switch {
	case nanos >= float64(math.MaxInt64):
		return math.MaxInt64
	case nanos < 1:
		return 1
	default:
		return int64(nanos)
	}
}

// limiterTolerance returns the GCRA burst tolerance in nanoseconds.
func limiterTolerance(interval int64, burst int) int64 {
	if burst <= 1 {
		return 0
	}

	tokens := int64(burst - 1)
	if tokens > math.MaxInt64/interval {
		return math.MaxInt64
	}

	return tokens * interval
}

// start launches the periodic drop reporter and, when enabled, the cached clock. It must be
// called once the gate starts.
func (r *connLimiter) start(ctx context.Context) {
	r.clock.Store(int64(time.Since(r.epoch)))
	r.wg.Go(func() { r.report(ctx) })

	if r.coarse {
		r.wg.Go(func() { r.clockLoop(ctx) })
	}
}

// stop stops the background goroutines and flushes the remaining drop counters.
func (r *connLimiter) stop() {
	r.wg.Wait()
	r.flush()
}

// clockLoop updates the cached clock used by the coarse-grained decision path.
func (r *connLimiter) clockLoop(ctx context.Context) {
	ticker := time.NewTicker(defaultLimiterClockInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.clock.Store(int64(time.Since(r.epoch)))
		}
	}
}

// report periodically flushes the per-shard drop counters until ctx is canceled.
func (r *connLimiter) report(ctx context.Context) {
	ticker := time.NewTicker(defaultLimiterReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.flush()
		}
	}
}

// flush reports the messages dropped since the previous flush and resets the counters.
func (r *connLimiter) flush() {
	var dropped int64

	for i := range r.shards {
		dropped += r.shards[i].dropped.Swap(0)
	}

	if dropped > 0 {
		log.Warnf("connection rate limit dropped %d messages in the last interval", dropped)
	}
}

// remove releases the bucket of cid.
func (r *connLimiter) remove(cid int64) {
	shard := r.loadShard(cid)

	if value, ok := shard.buckets.LoadAndDelete(cid); ok {
		shard.put(value.(*limiterBucket))
	}
}

// allow reports whether a message from cid may be processed and whether the connection should be
// closed because it reached the configured consecutive violation threshold.
//
// The bucket is created on first use, so a connection that never sends a business frame does not
// allocate one. The connection read loop owns the bucket until the connection is removed.
func (r *connLimiter) allow(cid int64) (allowed bool, closeConn bool) {
	shard := r.loadShard(cid)

	value, ok := shard.buckets.Load(cid)
	if !ok {
		candidate := shard.get()
		actual, loaded := shard.buckets.LoadOrStore(cid, candidate)
		if loaded {
			shard.put(candidate)
		}
		value = actual
	}
	bucket := value.(*limiterBucket)

	now := r.clock.Load()
	if !r.coarse || now == 0 {
		now = int64(time.Since(r.epoch))
	}

	switch {
	case now >= bucket.tat:
		bucket.tat = now + r.interval
	case bucket.tat-now <= r.tolerance:
		bucket.tat += r.interval
	default:
		if r.closeThreshold > 0 {
			bucket.violations++
			if int(bucket.violations) >= r.closeThreshold && !bucket.closing {
				bucket.closing = true
				closeConn = true
				log.Warnf("connection rate limit exceeded, closing connection, cid: %d, violations: %d",
					cid, bucket.violations)
			}
		}

		shard.dropped.Add(1)

		return false, closeConn
	}

	if bucket.violations != 0 {
		bucket.violations = 0
	}

	return true, false
}

// loadShard returns the shard of the given connection ID.
func (r *connLimiter) loadShard(cid int64) *limiterShard {
	return &r.shards[int(uint64(cid)&(defaultLimiterShardNum-1))]
}

// get returns a reset bucket from the shard free list, or a new one when the list is empty.
func (s *limiterShard) get() *limiterBucket {
	s.freeMu.Lock()
	bucket := s.free
	if bucket != nil {
		s.free = bucket.next
		bucket.next = nil
		s.freeCount--
	}
	s.freeMu.Unlock()

	if bucket != nil {
		return bucket
	}

	return &limiterBucket{}
}

// put resets bucket and returns it to the shard free list. When the list is full the bucket is
// dropped and left to the garbage collector.
func (s *limiterShard) put(bucket *limiterBucket) {
	bucket.tat = 0
	bucket.violations = 0
	bucket.closing = false
	bucket.next = nil

	s.freeMu.Lock()
	if s.freeCount < defaultLimiterFreeListSize {
		bucket.next = s.free
		s.free = bucket
		s.freeCount++
	}
	s.freeMu.Unlock()
}
