package memcache_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/lock/memcache/v2"
	"github.com/dobyte/due/v2/errors"
)

func TestLocker_Acquire(t *testing.T) {
	var (
		ctx    = context.Background()
		maker  = memcache.NewMaker()
		locker = maker.Make("lockName")
		other  = maker.Make("lockName")
	)

	if err := locker.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	// 锁被持有时，其他 Locker 无法获取
	if err := other.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got: %v", err)
	}

	if err := locker.Release(ctx); err != nil {
		t.Fatal(err)
	}

	// 释放后，其他 Locker 可以获取
	if err := other.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	defer other.Release(ctx)
}

func TestLocker_Parallel_Acquire(t *testing.T) {
	var (
		wg      sync.WaitGroup
		ctx     = context.Background()
		maker   = memcache.NewMaker()
		holders atomic.Int32
	)

	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			// 每个竞争者持有独立的 Locker(独立version)竞争同一把锁
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

			// 任意时刻只允许一个持有者进入临界区
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
