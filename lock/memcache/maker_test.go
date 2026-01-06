package memcache_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/lock/memcache/v2"
)

func TestLocker_Acquire(t *testing.T) {
	maker := memcache.NewMaker()

	locker := maker.Make("lockName")

	if err := locker.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	defer locker.Release(context.Background())

	time.Sleep(20 * time.Second)
}

func TestLocker_Parallel_Acquire(t *testing.T) {
	var (
		wg     sync.WaitGroup
		ctx    = context.Background()
		maker  = memcache.NewMaker()
		locker = maker.Make("lockName")
	)

	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if err := locker.Acquire(ctx); err != nil {
				t.Logf("%d acquire lock failed: %v", i, err)
				return
			}

			defer func() {
				if err := locker.Release(ctx); err != nil {
					t.Logf("%d release lock failed: %v", i, err)
				}
			}()

			t.Logf("%d do some things", i)

			time.Sleep(time.Second)
		}(i)
	}

	wg.Wait()
}
