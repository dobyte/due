package redis_test

import (
	"context"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/limiter/redis/v2"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
)

// requireRedis probes whether the local redis service is available and skips the test when it is
// not.
func requireRedis(tb testing.TB) {
	tb.Helper()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 200*time.Millisecond)
	if err != nil {
		tb.Skipf("local redis is not available: %v", err)
	}

	conn.Close()
}

// uniqueName returns a unique scene name so that tests do not share bucket state.
func uniqueName(tb testing.TB) string {
	tb.Helper()

	return tb.Name() + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func TestMaker_MakeInvalidRule(t *testing.T) {
	maker := redis.NewMaker()
	t.Cleanup(func() { _ = maker.Close() })

	if _, err := maker.Make("", limiter.Rule{Quota: 1}); !errors.Is(err, errors.ErrInvalidLimiterRule) {
		t.Fatalf("expect ErrInvalidLimiterRule for empty name, got: %v", err)
	}

	if _, err := maker.Make("invalid", limiter.Rule{}); !errors.Is(err, errors.ErrInvalidLimiterRule) {
		t.Fatalf("expect ErrInvalidLimiterRule for zero quota, got: %v", err)
	}
}

// TestMaker_MakeConflictingRule verifies that a scene keeps the rule it was created with, so the
// reported capacity and refill time always match the Lua script parameters of the shared key
// namespace.
func TestMaker_MakeConflictingRule(t *testing.T) {
	maker := redis.NewMaker()
	t.Cleanup(func() { _ = maker.Close() })

	if _, err := maker.Make("conflict", limiter.Rule{Quota: 1}); err != nil {
		t.Fatalf("make limiter failed: %v", err)
	}

	if _, err := maker.Make("conflict", limiter.Rule{Quota: 2}); !errors.Is(err, errors.ErrConflictLimiterRule) {
		t.Fatalf("expect ErrConflictLimiterRule, got: %v", err)
	}

	// A rule that normalizes to the initial one is accepted and shares the key namespace.
	if _, err := maker.Make("conflict", limiter.Rule{Quota: 1, Period: time.Second, Burst: 1}); err != nil {
		t.Fatalf("expect the identical rule to be accepted, got: %v", err)
	}
}

func TestLimiter_Allow(t *testing.T) {
	requireRedis(t)

	var (
		ctx   = context.Background()
		maker = redis.NewMaker()
	)
	t.Cleanup(func() { _ = maker.Close() })

	l, err := maker.Make(uniqueName(t), limiter.Rule{Quota: 2, Period: time.Hour})
	if err != nil {
		t.Fatal(err)
	}

	first, err := l.Allow(ctx, "uid:1")
	if err != nil {
		t.Fatal(err)
	}

	if !first.Allowed || first.Limit != 2 || first.Remaining != 1 {
		t.Fatalf("unexpected first result: %+v", first)
	}

	if first.ResetAfter <= 0 {
		t.Fatalf("expect a positive reset after, got: %v", first.ResetAfter)
	}

	second, err := l.Allow(ctx, "uid:1")
	if err != nil {
		t.Fatal(err)
	}

	if !second.Allowed || second.Remaining != 0 {
		t.Fatalf("unexpected second result: %+v", second)
	}

	third, err := l.Allow(ctx, "uid:1")
	if err != nil {
		t.Fatal(err)
	}

	if third.Allowed || third.Remaining != 0 || third.RetryAfter <= 0 {
		t.Fatalf("unexpected third result: %+v", third)
	}

	other, err := l.Allow(ctx, "uid:2")
	if err != nil {
		t.Fatal(err)
	}

	if !other.Allowed {
		t.Fatalf("expect the other key to be allowed, got: %+v", other)
	}

	scene, err := l.Allow(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	if !scene.Allowed {
		t.Fatalf("expect the scene key to be allowed, got: %+v", scene)
	}
}

func TestLimiter_Cost(t *testing.T) {
	requireRedis(t)

	var (
		ctx   = context.Background()
		maker = redis.NewMaker()
	)
	t.Cleanup(func() { _ = maker.Close() })

	l, err := maker.Make(uniqueName(t), limiter.Rule{Quota: 5, Period: time.Hour, Burst: 5})
	if err != nil {
		t.Fatal(err)
	}

	result, err := l.Allow(ctx, "key", 3)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Allowed || result.Remaining != 2 {
		t.Fatalf("unexpected first result: %+v", result)
	}

	result, err = l.Allow(ctx, "key", 3)
	if err != nil {
		t.Fatal(err)
	}

	if result.Allowed || result.Remaining != 2 {
		t.Fatalf("unexpected second result: %+v", result)
	}

	result, err = l.Allow(ctx, "key", 2)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Allowed || result.Remaining != 0 {
		t.Fatalf("unexpected third result: %+v", result)
	}

	if _, err = l.Allow(ctx, "key", 6); !errors.Is(err, errors.ErrInvalidLimiterCost) {
		t.Fatalf("expect ErrInvalidLimiterCost, got: %v", err)
	}
}

func TestLimiter_Refill(t *testing.T) {
	requireRedis(t)

	var (
		ctx   = context.Background()
		maker = redis.NewMaker()
	)
	t.Cleanup(func() { _ = maker.Close() })

	l, err := maker.Make(uniqueName(t), limiter.Rule{Quota: 10, Period: 100 * time.Millisecond, Burst: 1})
	if err != nil {
		t.Fatal(err)
	}

	if result, err := l.Allow(ctx, "key"); err != nil || !result.Allowed {
		t.Fatalf("expect the first request to be allowed, result: %+v, err: %v", result, err)
	}

	if result, err := l.Allow(ctx, "key"); err != nil || result.Allowed {
		t.Fatalf("expect the second request to be denied, result: %+v, err: %v", result, err)
	}

	time.Sleep(30 * time.Millisecond)

	if result, err := l.Allow(ctx, "key"); err != nil || !result.Allowed {
		t.Fatalf("expect the request after refill to be allowed, result: %+v, err: %v", result, err)
	}
}

func TestLimiter_Concurrent(t *testing.T) {
	requireRedis(t)

	var (
		ctx      = context.Background()
		maker    = redis.NewMaker()
		allowed  atomic.Int64
		wait     sync.WaitGroup
		routines = 200
	)
	t.Cleanup(func() { _ = maker.Close() })

	l, err := maker.Make(uniqueName(t), limiter.Rule{Quota: 100, Period: time.Hour})
	if err != nil {
		t.Fatal(err)
	}

	wait.Add(routines)

	for range routines {
		go func() {
			defer wait.Done()

			result, err := l.Allow(ctx, "key")
			if err != nil {
				t.Errorf("allow failed: %v", err)
				return
			}

			if result.Allowed {
				allowed.Add(1)
			}
		}()
	}

	wait.Wait()

	if got := allowed.Load(); got != 100 {
		t.Fatalf("expect exactly 100 allowed requests, got: %d", got)
	}
}

func TestMaker_Close(t *testing.T) {
	var (
		ctx   = context.Background()
		maker = redis.NewMaker()
	)

	l, err := maker.Make("close", limiter.Rule{Quota: 1, Period: time.Hour})
	if err != nil {
		t.Fatal(err)
	}

	if err = maker.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err = l.Allow(ctx, "key"); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation after close, got: %v", err)
	}

	if _, err = maker.Make("close", limiter.Rule{Quota: 1, Period: time.Hour}); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation from Make after close, got: %v", err)
	}

	if err = maker.Close(); err != nil {
		t.Fatalf("expect closing twice to return nil, got: %v", err)
	}
}

func TestLimiter_FailurePolicy(t *testing.T) {
	var (
		ctx   = context.Background()
		rules = limiter.Rule{Quota: 1, Period: time.Hour}
	)

	t.Run("FailOpen", func(t *testing.T) {
		maker := redis.NewMaker(
			redis.WithAddrs("127.0.0.1:1"),
			redis.WithTimeout(50*time.Millisecond),
			redis.WithFailurePolicy(limiter.FailOpen),
		)
		t.Cleanup(func() { _ = maker.Close() })

		l, err := maker.Make("failure-open", rules)
		if err != nil {
			t.Fatal(err)
		}

		result, err := l.Allow(ctx, "key")
		if err != nil {
			t.Fatal(err)
		}

		if !result.Allowed || !result.Degraded {
			t.Fatalf("expect a degraded allow, got: %+v", result)
		}
	})

	t.Run("FailClosed", func(t *testing.T) {
		maker := redis.NewMaker(
			redis.WithAddrs("127.0.0.1:1"),
			redis.WithTimeout(50*time.Millisecond),
			redis.WithFailurePolicy(limiter.FailClosed),
		)
		t.Cleanup(func() { _ = maker.Close() })

		l, err := maker.Make("failure-closed", rules)
		if err != nil {
			t.Fatal(err)
		}

		if _, err = l.Allow(ctx, "key"); err == nil {
			t.Fatal("expect an error from the fail-closed policy")
		}
	})
}
