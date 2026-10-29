package memory_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/dobyte/due/v2/limiter"
	"github.com/dobyte/due/v2/limiter/memory"
)

func BenchmarkLimiter_Allow(b *testing.B) {
	var (
		ctx   = context.Background()
		maker = memory.NewMaker()
		keys  = make([]string, 1024)
	)

	b.Cleanup(func() { _ = maker.Close() })

	for i := range len(keys) {
		keys[i] = "uid:" + strconv.Itoa(i)
	}

	l, err := maker.Make("bench", limiter.Rule{
		Quota:  1e9,
		Period: time.Hour,
		Burst:  1_000_000_000,
	})
	if err != nil {
		b.Fatal(err)
	}

	// Prewarm buckets so the timed loops do not include bucket creation.
	_, _ = l.Allow(ctx, "")
	_, _ = l.Allow(ctx, "uid:1")
	for _, key := range keys {
		_, _ = l.Allow(ctx, key)
	}

	b.Run("scene-key", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, _ = l.Allow(ctx, "")
		}
	})

	b.Run("same-user", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, _ = l.Allow(ctx, "uid:1")
		}
	})

	b.Run("different-users", func(b *testing.B) {
		b.ReportAllocs()

		for i := range b.N {
			_, _ = l.Allow(ctx, keys[i&1023])
		}
	})

	b.Run("same-user-parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = l.Allow(ctx, "uid:1")
			}
		})
	})

	b.Run("different-users-parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_, _ = l.Allow(ctx, keys[i&1023])
				i = (i + 1) & 1023
			}
		})
	})
}
