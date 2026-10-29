package memory

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/limiter"
)

func BenchmarkMaker_EvictScan(b *testing.B) {
	var (
		ctx   = context.Background()
		sizes = []int{1_000, 10_000, 100_000, 1_000_000}
	)

	for _, size := range sizes {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			maker := NewMaker(WithCleanupInterval(time.Hour))
			b.Cleanup(func() { _ = maker.Close() })

			l, err := maker.Make("evict-scan", limiter.Rule{Quota: 1, Period: time.Hour, Burst: 1})
			if err != nil {
				b.Fatal(err)
			}

			for i := range size {
				if _, err = l.Allow(ctx, strconv.Itoa(i)); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()

			for b.Loop() {
				maker.evict()
			}
		})
	}
}

func BenchmarkLimiter_AllowWithEvict(b *testing.B) {
	const (
		keyCount      = 100_000
		evictInterval = 100 * time.Millisecond
	)

	var (
		ctx   = context.Background()
		maker = NewMaker(WithCleanupInterval(time.Hour))
	)

	l, err := maker.Make("evict-load", limiter.Rule{
		Quota:  1e9,
		Period: time.Hour,
		Burst:  1_000_000_000,
	})
	if err != nil {
		b.Fatal(err)
	}

	ml := l.(*Limiter)

	for i := range keyCount {
		if _, err = ml.Allow(ctx, strconv.Itoa(i)); err != nil {
			b.Fatal(err)
		}
	}

	// Prewarm the hot key used by the parallel loop.
	if _, err = ml.Allow(ctx, "hot"); err != nil {
		b.Fatal(err)
	}

	stop := make(chan struct{})
	var wait sync.WaitGroup

	wait.Add(1)
	go func() {
		defer wait.Done()

		ticker := time.NewTicker(evictInterval)
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

	b.Cleanup(func() {
		close(stop)
		wait.Wait()
		_ = maker.Close()
	})

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ml.Allow(ctx, "hot")
		}
	})
}
