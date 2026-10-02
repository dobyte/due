package memcache_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gomemcache "github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/lock/memcache/v2"
	"github.com/dobyte/due/v2/errors"
)

// requireMemcache probes whether a local memcached service is available and skips the test when it
// is not.
func requireMemcache(t *testing.T) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:11211", 200*time.Millisecond)
	if err != nil {
		t.Skipf("local memcached is not available: %v", err)
	}

	conn.Close()
}

func TestLocker_Acquire(t *testing.T) {
	requireMemcache(t)

	var (
		ctx    = context.Background()
		maker  = memcache.NewMaker()
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

	// After the release, another Locker can acquire it.
	if err := other.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	defer other.Release(ctx)
}

func TestLocker_Parallel_Acquire(t *testing.T) {
	requireMemcache(t)

	var (
		wg      sync.WaitGroup
		ctx     = context.Background()
		maker   = memcache.NewMaker()
		holders atomic.Int32
	)

	t.Cleanup(func() { _ = maker.Close() })

	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			// Each contender holds an independent Locker (an independent version) competing for the
			// same lock.
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

			// At most one holder is allowed to enter the critical section at any time.
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
	requireMemcache(t)

	var (
		ctx    = context.Background()
		maker  = memcache.NewMaker(memcache.WithExpiration(3 * time.Second))
		locker = maker.Make("lockRenewal")
		other  = maker.Make("lockRenewal")
	)

	t.Cleanup(func() { _ = maker.Close() })

	if err := locker.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	defer locker.Release(ctx)

	// The wait exceeds the natural expiration of the lock; if the background renewal did not work,
	// the lock would have expired long ago and another holder should be able to acquire it.
	// (memcached records the expiration in whole seconds, so a 3s lifetime may actually be 2~3s.)
	time.Sleep(4500 * time.Millisecond)

	if err := other.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation since the lock is renewed, got: %v", err)
	}
}

func TestLocker_Expired_Release(t *testing.T) {
	requireMemcache(t)

	var (
		ctx    = context.Background()
		maker  = memcache.NewMaker()
		locker = maker.Make("lockExpired")
	)

	t.Cleanup(func() { _ = maker.Close() })

	// Acquire the lock with a fixed expiration and without background renewal.
	// (memcached expiration has second granularity, so the fixed expiration is rounded to 1s.)
	if err := locker.TryAcquire(ctx, time.Second); err != nil {
		t.Fatal(err)
	}

	// Wait for the lock to expire naturally before releasing; the release should detect that the
	// lock is lost.
	time.Sleep(1600 * time.Millisecond)

	if err := locker.Release(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation when release an expired lock, got: %v", err)
	}
}

func TestMaker_Closed(t *testing.T) {
	requireMemcache(t)

	var (
		ctx = context.Background()
		// An external client whose lifecycle is not managed by the Maker, used to verify that lock
		// acquisition fails fast after Close.
		// (The external client is still usable after Maker.Close, so the acquisition would succeed
		// if it were not explicitly intercepted.)
		client = gomemcache.New("127.0.0.1:11211")
		maker  = memcache.NewMaker(memcache.WithClient(client))
		locker = maker.Make("lockClosed")
	)

	t.Cleanup(func() { _ = client.Close() })

	if err := maker.Close(); err != nil {
		t.Fatal(err)
	}

	// After Close, lock acquisition should fail fast rather than succeed with a background renewal
	// that silently stops because the lifecycle context was canceled.
	if err := locker.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation after maker closed, got: %v", err)
	}
}
