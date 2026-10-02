package redis_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/lock/redis/v2"
	"github.com/dobyte/due/v2/errors"
	goredis "github.com/redis/go-redis/v9"
)

// requireRedis probes whether the local redis service is available and skips the test when it is
// not.
func requireRedis(t *testing.T) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 200*time.Millisecond)
	if err != nil {
		t.Skipf("local redis is not available: %v", err)
	}

	conn.Close()
}

func TestLocker_Acquire(t *testing.T) {
	requireRedis(t)

	var (
		ctx    = context.Background()
		maker  = redis.NewMaker()
		locker = maker.Make("lockName")
		other  = maker.Make("lockName")
	)

	t.Cleanup(func() { _ = maker.Close() })

	if err := locker.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	// While the lock is held, another Locker cannot acquire it.
	if err := other.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got: %v", err)
	}

	if err := locker.Release(ctx); err != nil {
		t.Fatal(err)
	}

	// After the lock is released, another Locker can acquire it.
	if err := other.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	defer other.Release(ctx)
}

func TestLocker_Parallel_Acquire(t *testing.T) {
	requireRedis(t)

	var (
		wg      sync.WaitGroup
		ctx     = context.Background()
		maker   = redis.NewMaker()
		holders atomic.Int32
	)

	t.Cleanup(func() { _ = maker.Close() })

	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			// Every contender holds its own Locker (with its own version) and competes for the same lock.
			locker := maker.Make("lockName")

			if err := locker.Acquire(ctx); err != nil {
				t.Errorf("%d acquire lock failed: %v", i, err)
				return
			}

			defer func() {
				if err := locker.Release(ctx); err != nil {
					t.Errorf("%d release lock failed: %v", i, err)
				}
			}()

			// At most one holder may enter the critical section at any time.
			if n := holders.Add(1); n != 1 {
				t.Errorf("%d lock is not exclusive, concurrent holders: %d", i, n)
			}

			t.Logf("%d do some things", i)

			time.Sleep(100 * time.Millisecond)

			holders.Add(-1)
		}(i)
	}

	wg.Wait()
}

func TestLocker_Renewal(t *testing.T) {
	requireRedis(t)

	var (
		ctx    = context.Background()
		maker  = redis.NewMaker(redis.WithExpiration(500 * time.Millisecond))
		locker = maker.Make("lockRenewal")
		other  = maker.Make("lockRenewal")
	)

	t.Cleanup(func() { _ = maker.Close() })

	if err := locker.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	defer locker.Release(ctx)

	// The wait exceeds the default expiration; if the background renewal did not work, the lock
	// would have expired long ago and another Locker should be able to acquire it.
	time.Sleep(1200 * time.Millisecond)

	if err := other.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation since the lock is renewed, got: %v", err)
	}
}

func TestLocker_Expired_Release(t *testing.T) {
	requireRedis(t)

	var (
		ctx    = context.Background()
		maker  = redis.NewMaker()
		locker = maker.Make("lockExpired")
	)

	t.Cleanup(func() { _ = maker.Close() })

	// Acquire the lock with a fixed expiration without starting the background renewal.
	if err := locker.TryAcquire(ctx, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// Wait for the lock to expire naturally and then release it; the lost lock should be detected.
	time.Sleep(400 * time.Millisecond)

	if err := locker.Release(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation when release an expired lock, got: %v", err)
	}
}

func TestMaker_Closed(t *testing.T) {
	requireRedis(t)

	var (
		ctx = context.Background()
		// External client whose lifecycle is not managed by the Maker; it is used to verify that
		// acquiring a lock fails fast after Close.
		// (With a built-in client the closed connection pool already reports an error, whereas an
		// external client needs an explicit check.)
		client = goredis.NewUniversalClient(&goredis.UniversalOptions{Addrs: []string{"127.0.0.1:6379"}})
		maker  = redis.NewMaker(redis.WithClient(client))
		locker = maker.Make("lockClosed")
	)

	t.Cleanup(func() { _ = client.Close() })

	if err := maker.Close(); err != nil {
		t.Fatal(err)
	}

	// Acquiring a lock after Close should fail fast instead of succeeding and then silently losing
	// the background renewal because the lifecycle context was canceled.
	if err := locker.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation after maker closed, got: %v", err)
	}
}
