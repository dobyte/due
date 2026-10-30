package gate

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/packet"
)

func TestConnectionLimiter(t *testing.T) {
	r := newConnLimiter(&limiterOptions{quota: 1, period: time.Second, burst: 1})

	// The first message creates a full bucket and is allowed.
	if allowed, closeConn := r.allow(1); !allowed || closeConn {
		t.Fatal("the first message must be allowed")
	}
	if _, ok := r.loadShard(1).buckets.Load(int64(1)); !ok {
		t.Fatal("the bucket must be created on first use")
	}
	if allowed, closeConn := r.allow(1); allowed || closeConn {
		t.Fatal("the second message must be limited")
	}
	if got := r.loadShard(1).dropped.Load(); got != 1 {
		t.Fatalf("unexpected dropped count: %d", got)
	}

	// Removing and recreating the bucket resets it.
	before, _ := r.loadShard(1).buckets.Load(int64(1))
	r.remove(1)

	if allowed, closeConn := r.allow(1); !allowed || closeConn {
		t.Fatal("a recreated bucket must start full")
	}

	if after, ok := r.loadShard(1).buckets.Load(int64(1)); !ok || after != before {
		t.Fatal("the bucket must be reused from the shard free list")
	}
}

func TestLimiterShardFreeListLimit(t *testing.T) {
	var shard limiterShard

	for range defaultLimiterFreeListSize + 1 {
		shard.put(&limiterBucket{})
	}

	if got := shard.freeCount; got != defaultLimiterFreeListSize {
		t.Fatalf("unexpected free list size: %d", got)
	}

	for range defaultLimiterFreeListSize {
		if shard.get() == nil {
			t.Fatal("the free list must return a bucket")
		}
	}

	if got := shard.freeCount; got != 0 {
		t.Fatalf("the free list must be empty: %d", got)
	}
}

func TestLimiterShardCacheLineAlignment(t *testing.T) {
	const cacheLineSize = 64

	if size := unsafe.Sizeof(limiterShard{}); size%cacheLineSize != 0 {
		t.Fatalf("limiterShard size %d is not a multiple of the cache line size", size)
	}

	var (
		shard limiterShard
		r     connLimiter
	)

	if off := unsafe.Offsetof(shard.dropped); off%cacheLineSize != 0 {
		t.Fatalf("dropped offset %d is not cache line aligned", off)
	}
	if off := unsafe.Offsetof(shard.freeMu); off%cacheLineSize != 0 {
		t.Fatalf("free list offset %d is not cache line aligned", off)
	}
	if off := unsafe.Offsetof(r.shards); off%cacheLineSize != 0 {
		t.Fatalf("shards offset %d is not cache line aligned", off)
	}
}

func TestConnectionLimiterClosesAfterThreshold(t *testing.T) {
	r := newConnLimiter(&limiterOptions{
		quota: 1, period: time.Hour, burst: 1, closeThreshold: 3,
	})

	if allowed, closeConn := r.allow(1); !allowed || closeConn {
		t.Fatal("the first message must be allowed")
	}

	for range 2 {
		if allowed, closeConn := r.allow(1); allowed || closeConn {
			t.Fatal("the connection must not close before the threshold")
		}
	}

	if allowed, closeConn := r.allow(1); allowed || !closeConn {
		t.Fatal("the connection must close at the threshold")
	}

	// The close signal is throttled by the retry interval while a close is pending.
	if allowed, closeConn := r.allow(1); allowed || closeConn {
		t.Fatal("the close signal must not repeat within the retry interval")
	}
}

func TestConnectionLimiterRetriesCloseSignal(t *testing.T) {
	r := newConnLimiter(&limiterOptions{
		quota: 1, period: time.Hour, burst: 1, closeThreshold: 1,
	})
	r.closeRetry = 10 * time.Millisecond

	if allowed, closeConn := r.allow(1); !allowed || closeConn {
		t.Fatal("the first message must be allowed")
	}

	if allowed, closeConn := r.allow(1); allowed || !closeConn {
		t.Fatal("the connection must close at the threshold")
	}

	if allowed, closeConn := r.allow(1); allowed || closeConn {
		t.Fatal("the close signal must not repeat within the retry interval")
	}

	// A close that did not take effect must be retried after the retry interval.
	time.Sleep(15 * time.Millisecond)

	if allowed, closeConn := r.allow(1); allowed || !closeConn {
		t.Fatal("a failed close must be retried after the retry interval")
	}

	// A reused bucket starts with a clean retry state.
	shard := &limiterShard{}
	shard.put(&limiterBucket{violations: 3, retryAt: 1})

	if bucket := shard.get(); bucket.violations != 0 || bucket.retryAt != 0 {
		t.Fatalf("the bucket state was not reset: violations=%d retryAt=%d", bucket.violations, bucket.retryAt)
	}
}

func TestConnectionLimiterBurst(t *testing.T) {
	r := newConnLimiter(&limiterOptions{
		quota: 1, period: time.Hour, burst: 3,
	})

	for i := range 3 {
		if allowed, _ := r.allow(1); !allowed {
			t.Fatalf("message %d must be allowed", i+1)
		}
	}

	if allowed, _ := r.allow(1); allowed {
		t.Fatal("the message after the burst must be limited")
	}
}

func TestConnectionLimiterRefill(t *testing.T) {
	r := newConnLimiter(&limiterOptions{
		quota: 100, period: time.Second, burst: 1,
	})

	if allowed, _ := r.allow(1); !allowed {
		t.Fatal("the first message must be allowed")
	}
	if allowed, _ := r.allow(1); allowed {
		t.Fatal("the second message must be limited")
	}

	// The emission interval is 10ms.
	time.Sleep(25 * time.Millisecond)

	if allowed, _ := r.allow(1); !allowed {
		t.Fatal("the message after refill must be allowed")
	}
}

func TestConnectionLimiterCoarseClock(t *testing.T) {
	r := newConnLimiter(&limiterOptions{
		quota: 100, period: time.Second, burst: 1,
	})
	if !r.coarse {
		t.Fatal("a 10ms interval must use the cached clock")
	}

	r.clock.Store(int64(time.Second))
	if allowed, _ := r.allow(1); !allowed {
		t.Fatal("the first message must be allowed")
	}

	r.clock.Store(int64(2 * time.Second))
	if allowed, _ := r.allow(1); !allowed {
		t.Fatal("the cached clock must refill the bucket")
	}
}

func TestLimiterOptionsNormalize(t *testing.T) {
	tests := []struct {
		name    string
		give    limiterOptions
		want    limiterOptions
		enabled bool
	}{
		{
			name: "disabled without a quota",
			give: limiterOptions{},
			want: limiterOptions{},
		},
		{
			name:    "fills the defaults",
			give:    limiterOptions{quota: 2},
			want:    limiterOptions{quota: 2, period: time.Second, burst: 2},
			enabled: true,
		},
		{
			name:    "keeps explicit values",
			give:    limiterOptions{quota: 2, period: time.Minute, burst: 5},
			want:    limiterOptions{quota: 2, period: time.Minute, burst: 5},
			enabled: true,
		},
		{
			name:    "rounds a fractional quota up",
			give:    limiterOptions{quota: 0.5},
			want:    limiterOptions{quota: 0.5, period: time.Second, burst: 1},
			enabled: true,
		},
		{
			name:    "clamps a negative close threshold",
			give:    limiterOptions{quota: 2, closeThreshold: -1},
			want:    limiterOptions{quota: 2, period: time.Second, burst: 2},
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.give
			if got := opts.normalize(); got != tt.enabled {
				t.Fatalf("unexpected enabled state: %v", got)
			}
			if opts != tt.want {
				t.Fatalf("unexpected options: %+v", opts)
			}
		})
	}
}

func TestGateLimiterDisabledByDefault(t *testing.T) {
	o := defaultOptions()
	if o.limiter == nil {
		t.Fatal("limiter options must be initialized")
	}
	if o.limiter.normalize() {
		t.Fatalf("rate limiting must be disabled by default: %+v", o.limiter)
	}
}

func TestGateLimiterOptions(t *testing.T) {
	o := defaultOptions()
	WithLimiterPeriod(2 * time.Second)(o)
	WithLimiterBurst(4)(o)
	WithLimiterCloseThreshold(3)(o)
	WithLimiterQuota(2)(o)

	if o.limiter.quota != 2 || o.limiter.period != 2*time.Second ||
		o.limiter.burst != 4 || o.limiter.closeThreshold != 3 {
		t.Fatalf("unexpected limiter options: %+v", o.limiter)
	}

	// A zero close threshold disables closing.
	o = defaultOptions()
	WithLimiterCloseThreshold(0)(o)
	if o.limiter.closeThreshold != 0 {
		t.Fatalf("a zero close threshold must disable closing: %+v", o.limiter)
	}

	// An invalid quota is ignored and keeps the previous value.
	o = defaultOptions()
	WithLimiterQuota(5)(o)
	WithLimiterQuota(-1)(o)
	if o.limiter.quota != 5 {
		t.Fatalf("an invalid quota must be ignored: %+v", o.limiter)
	}

	// A zero quota explicitly disables a configured limiter.
	o = defaultOptions()
	o.limiter.quota = 10
	WithLimiterQuota(0)(o)
	if o.limiter.quota != 0 || o.limiter.normalize() {
		t.Fatalf("a zero quota must disable the limiter: %+v", o.limiter)
	}
}

func TestGateConnLimiter(t *testing.T) {
	opts := &limiterOptions{quota: 2}
	if !opts.normalize() {
		t.Fatal("rate limiting must be enabled")
	}

	l := newConnLimiter(opts)
	interval := int64(time.Second / 2)
	if l.interval != interval || l.tolerance != interval {
		t.Fatalf("unexpected limiter: interval = %d, tolerance = %d", l.interval, l.tolerance)
	}
}

func TestGateHandleReceiveLimited(t *testing.T) {
	f := newGateFixture(t)
	f.gate.limiter = newConnLimiter(&limiterOptions{quota: 1, period: time.Second, burst: 1})
	f.start(t)

	conn := newGateConn(2001, 0)
	f.server.connect(conn)

	if _, ok := f.gate.limiter.loadShard(conn.ID()).buckets.Load(conn.ID()); ok {
		t.Fatal("the bucket must not be created on connect")
	}

	buf, err := packet.PackMessage(1, 1, []byte("payload"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.server.receive(conn, buf)

	if _, ok := f.gate.limiter.loadShard(conn.ID()).buckets.Load(conn.ID()); !ok {
		t.Fatal("the bucket must be created on first receive")
	}

	buf, err = packet.PackMessage(1, 1, []byte("payload"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.server.receive(conn, buf)

	if got := f.gate.limiter.loadShard(conn.ID()).dropped.Load(); got != 1 {
		t.Fatalf("expect one limited message, got: %d", got)
	}
	if conn.closed.Load() {
		t.Fatal("a limited connection must not be closed")
	}
	if ok, _ := f.gate.session.has(cluster.Conn, conn.ID()); !ok {
		t.Fatal("a limited connection must stay registered")
	}

	// The bucket is released with the connection.
	f.server.disconnect(conn)

	if _, ok := f.gate.limiter.loadShard(conn.ID()).buckets.Load(conn.ID()); ok {
		t.Fatal("the bucket must be removed after disconnect")
	}
}

func TestGateHandleReceiveClosesLimitedConn(t *testing.T) {
	f := newGateFixture(t)
	f.gate.limiter = newConnLimiter(&limiterOptions{
		quota: 1, period: time.Hour, burst: 1, closeThreshold: 2,
	})
	f.start(t)

	conn := newGateConn(2002, 0)
	f.server.connect(conn)

	for range 3 {
		buf, err := packet.PackMessage(1, 1, []byte("payload"))
		if err != nil {
			t.Fatalf("pack message failed: %v", err)
		}
		f.server.receive(conn, buf)
	}

	waitForCondition(t, time.Second, conn.closed.Load, "the connection must be closed after reaching the threshold")

	f.server.disconnect(conn)
}

// deferredCloseConn models a transport whose Close waits for the receive callback to return.
type deferredCloseConn struct {
	*gateConn
	receiveReturned chan struct{}
}

func (c *deferredCloseConn) Close(force ...bool) error {
	<-c.receiveReturned
	c.closed.Store(true)
	return nil
}

func TestGateHandleReceiveClosesLimitedConnOutsideCallback(t *testing.T) {
	f := newGateFixture(t)
	f.gate.limiter = newConnLimiter(&limiterOptions{
		quota: 1, period: time.Hour, burst: 1, closeThreshold: 1,
	})
	f.start(t)

	conn := &deferredCloseConn{
		gateConn:        newGateConn(2003, 0),
		receiveReturned: make(chan struct{}),
	}
	f.server.connect(conn)

	var once sync.Once
	releaseReceive := func() { once.Do(func() { close(conn.receiveReturned) }) }
	t.Cleanup(releaseReceive)
	t.Cleanup(func() { f.server.disconnect(conn) })

	// The first message fills the bucket and is delivered.
	buf, err := packet.PackMessage(1, 1, []byte("payload"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.server.receive(conn, buf)

	// The second message exceeds the limit and requests the close. Closing inside the callback
	// would make the transport wait for the callback to return, so the handler must defer it.
	buf, err = packet.PackMessage(1, 1, []byte("payload"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		f.server.receive(conn, buf)
	}()

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("the receive handler blocked on the connection close")
	}

	// The close may only complete after the receive callback has returned.
	releaseReceive()

	waitForCondition(t, time.Second, conn.closed.Load, "the connection was not closed")
}

func BenchmarkConnLimiterAllow(b *testing.B) {
	r := newConnLimiter(&limiterOptions{quota: 1e9, period: time.Second, burst: 1e9})
	r.allow(1)

	b.ReportAllocs()

	for b.Loop() {
		r.allow(1)
	}
}

func BenchmarkConnLimiterDrop(b *testing.B) {
	r := newConnLimiter(&limiterOptions{quota: 1, period: time.Hour, burst: 1})
	r.allow(1)

	b.ReportAllocs()

	for b.Loop() {
		r.allow(1)
	}
}

func BenchmarkConnLimiterAllowParallel(b *testing.B) {
	r := newConnLimiter(&limiterOptions{quota: 1e9, period: time.Second, burst: 1e9})

	var cid atomic.Int64

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		id := cid.Add(1)
		r.allow(id)

		for pb.Next() {
			r.allow(id)
		}
	})
}

func BenchmarkConnLimiterDropParallel(b *testing.B) {
	r := newConnLimiter(&limiterOptions{quota: 1, period: time.Hour, burst: 1})

	var cid atomic.Int64

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		id := cid.Add(1)
		r.allow(id)

		for pb.Next() {
			r.allow(id)
		}
	})
}

func BenchmarkConnLimiterAllowCoarseClock(b *testing.B) {
	r := newConnLimiter(&limiterOptions{quota: 20, period: time.Second, burst: 1e9})
	ctx, cancel := context.WithCancel(context.Background())

	r.start(ctx)
	defer func() {
		cancel()
		r.stop()
	}()
	r.allow(1)

	b.ReportAllocs()

	for b.Loop() {
		r.allow(1)
	}
}

func BenchmarkConnLimiterLazyCreate(b *testing.B) {
	r := newConnLimiter(&limiterOptions{quota: 1e9, period: time.Second, burst: 1e9})

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		cid := int64(i + 1)
		r.allow(cid)
		r.remove(cid)
	}
}
