package memcache_test

import (
	"context"
	"github.com/dobyte/due/lock/memcache/v2"
	"sync"
	"testing"
	"time"
)

func TestMaker_Make(t *testing.T) {
	maker := memcache.NewMaker()

	locker := maker.Make("lockName")

	if err := locker.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	defer locker.Release(context.Background())

	time.Sleep(20 * time.Second)
}

func TestMaker_Parallel_Make(t *testing.T) {
	var (
		wg     sync.WaitGroup
		ctx    = context.Background()
		maker  = memcache.NewMaker()
		locker = maker.Make("lockName")
	)

	for i := 0; i < 100; i++ {
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
