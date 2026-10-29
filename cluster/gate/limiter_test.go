package gate

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/packet"
)

func TestConnectionLimiter(t *testing.T) {
	r := newConnLimiter(&limiterOptions{quota: 1, period: time.Second, burst: 1})

	// A missing bucket does not drop messages.
	if allowed, closeConn := r.allow(1); !allowed || closeConn {
		t.Fatal("a connection without a bucket must be allowed")
	}

	r.add(1)

	if allowed, closeConn := r.allow(1); !allowed || closeConn {
		t.Fatal("the first message must be allowed")
	}
	if allowed, closeConn := r.allow(1); allowed || closeConn {
		t.Fatal("the second message must be limited")
	}
	if got := r.dropped.Load(); got != 1 {
		t.Fatalf("unexpected dropped count: %d", got)
	}

	// Removing and re-adding the bucket resets it.
	r.remove(1)
	r.add(1)

	if allowed, closeConn := r.allow(1); !allowed || closeConn {
		t.Fatal("a recreated bucket must start full")
	}
}

func TestConnectionLimiterClosesAfterThreshold(t *testing.T) {
	r := newConnLimiter(&limiterOptions{
		quota: 1, period: time.Hour, burst: 1, closeThreshold: 3,
	})
	r.add(1)

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

	// The close signal is only emitted once.
	if allowed, closeConn := r.allow(1); allowed || closeConn {
		t.Fatal("the close signal must not repeat")
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
	if l.burst != 2 || l.rate != 2 {
		t.Fatalf("unexpected limiter: burst = %d, rate = %f", l.burst, l.rate)
	}
}

func TestGateHandleReceiveLimited(t *testing.T) {
	f := newGateFixture(t)
	f.gate.limiter = newConnLimiter(&limiterOptions{quota: 1, period: time.Second, burst: 1})
	f.start(t)

	conn := newGateConn(2001, 0)
	f.server.connect(conn)

	buf, err := packet.PackMessage(1, 1, []byte("payload"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.server.receive(conn, buf)

	buf, err = packet.PackMessage(1, 1, []byte("payload"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.server.receive(conn, buf)

	if got := f.gate.limiter.dropped.Load(); got != 1 {
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

	if bucket := f.gate.limiter.loadShard(conn.ID()).buckets[conn.ID()]; bucket != nil {
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

	if !conn.closed.Load() {
		t.Fatal("the connection must be closed after reaching the threshold")
	}

	f.server.disconnect(conn)
}
