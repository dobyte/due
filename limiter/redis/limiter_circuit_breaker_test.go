package redis

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/limiter"
)

func TestLimiter_CircuitBreaker(t *testing.T) {
	maker := NewMaker(
		WithAddrs("127.0.0.1:1"),
		WithTimeout(20*time.Millisecond),
		WithMaxRetries(-1),
		WithFailurePolicy(limiter.FailOpen),
	)
	maker.openDuration = 500 * time.Millisecond
	t.Cleanup(func() { _ = maker.Close() })

	l, err := maker.Make("circuit", limiter.Rule{Quota: 1, Period: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < circuitBreakerThreshold; i++ {
		result, err := l.Allow(context.Background(), "uid:1")
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !result.Degraded {
			t.Fatalf("allow %d: expect degraded result, got: %+v", i, result)
		}
	}

	if !maker.open.Load() {
		t.Fatal("expect circuit to be open")
	}

	result, err := l.Allow(context.Background(), "uid:1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Degraded {
		t.Fatalf("expect degraded result, got: %+v", result)
	}

	deadline := time.Now().Add(2 * time.Second)
	for maker.open.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if maker.open.Load() {
		t.Fatal("expect circuit to close after cooldown")
	}
}
