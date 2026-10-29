package memory

import (
	"context"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/limiter"
)

// TestStressAllowWithEvict measures hot-key Allow latency with and without concurrent eviction.
//
// It is opt-in because it allocates latency samples for every operation:
//
//	DUE_LIMITER_STRESS=1 go test -run TestStressAllowWithEvict -v ./limiter/memory
func TestStressAllowWithEvict(t *testing.T) {
	if os.Getenv("DUE_LIMITER_STRESS") == "" {
		t.Skip("set DUE_LIMITER_STRESS=1 to run")
	}

	const (
		keyCount     = 100_000
		workers      = 16
		opsPerWorker = 200_000
		batchSize    = 1_000
	)

	batchesPerWorker := opsPerWorker / batchSize

	run := func(t *testing.T, withEvict bool) {
		t.Helper()

		maker := NewMaker(WithCleanupInterval(time.Hour))
		defer maker.Close()

		l, err := maker.Make("stress", limiter.Rule{
			Quota:  1e9,
			Period: time.Hour,
			Burst:  1_000_000_000,
		})
		if err != nil {
			t.Fatal(err)
		}

		var (
			ctx = context.Background()
			ml  = l.(*Limiter)
		)

		for i := range keyCount {
			if _, err = ml.Allow(ctx, strconv.Itoa(i)); err != nil {
				t.Fatal(err)
			}
		}

		if _, err = ml.Allow(ctx, "hot"); err != nil {
			t.Fatal(err)
		}

		stop := make(chan struct{})
		var evictWait sync.WaitGroup

		if withEvict {
			evictWait.Add(1)

			go func() {
				defer evictWait.Done()

				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-stop:
						return
					case <-ticker.C:
						maker.evict()
					}
				}
			}()
		}

		var (
			latencies = make([][]time.Duration, workers)
			wait      sync.WaitGroup
		)

		wait.Add(workers)
		start := time.Now()

		for w := range workers {
			latencies[w] = make([]time.Duration, batchesPerWorker)

			go func(w int) {
				defer wait.Done()

				for i := range batchesPerWorker {
					begin := time.Now()

					for range batchSize {
						_, _ = ml.Allow(ctx, "hot")
					}

					latencies[w][i] = time.Since(begin) / batchSize
				}
			}(w)
		}

		wait.Wait()
		elapsed := time.Since(start)

		close(stop)
		evictWait.Wait()

		all := make([]time.Duration, 0, workers*batchesPerWorker)
		for _, samples := range latencies {
			all = append(all, samples...)
		}

		sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

		percentile := func(percent float64) time.Duration {
			return all[int(float64(len(all)-1)*percent)]
		}

		name := "baseline"
		if withEvict {
			name = "with-evict"
		}

		t.Logf("%s workers=%d ops=%d batch=%d throughput=%.0f ops/s batch-p50=%v batch-p95=%v batch-p99=%v batch-max=%v",
			name,
			workers,
			workers*opsPerWorker,
			batchSize,
			float64(workers*opsPerWorker)/elapsed.Seconds(),
			percentile(0.50),
			percentile(0.95),
			percentile(0.99),
			all[len(all)-1],
		)
	}

	t.Run("baseline", func(t *testing.T) { run(t, false) })
	t.Run("with-evict", func(t *testing.T) { run(t, true) })
}
