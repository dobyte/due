package redis_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/dobyte/due/limiter/redis/v2"
	"github.com/dobyte/due/v2/limiter"
)

func BenchmarkLimiter_Allow(b *testing.B) {
	requireRedis(b)

	var (
		ctx   = context.Background()
		maker = redis.NewMaker()
		keys  = make([]string, 1024)
	)

	b.Cleanup(func() { _ = maker.Close() })

	for i := range len(keys) {
		keys[i] = "uid:" + strconv.Itoa(i)
	}

	l, err := maker.Make(uniqueName(b), limiter.Rule{
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

func BenchmarkLimiter_Timeout(b *testing.B) {
	requireRedis(b)

	var (
		ctx          = context.Background()
		withMaker    = redis.NewMaker()
		withoutMaker = redis.NewMaker(redis.WithTimeout(0))
		rule         = limiter.Rule{Quota: 1e9, Period: time.Hour, Burst: 1_000_000_000}
	)

	b.Cleanup(func() {
		_ = withMaker.Close()
		_ = withoutMaker.Close()
	})

	withTimeout, err := withMaker.Make(uniqueName(b)+"-with", rule)
	if err != nil {
		b.Fatal(err)
	}

	withoutTimeout, err := withoutMaker.Make(uniqueName(b)+"-without", rule)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("with-timeout", func(b *testing.B) {
		_, _ = withTimeout.Allow(ctx, "key")
		b.ReportAllocs()

		for b.Loop() {
			_, _ = withTimeout.Allow(ctx, "key")
		}
	})

	b.Run("without-timeout", func(b *testing.B) {
		_, _ = withoutTimeout.Allow(ctx, "key")
		b.ReportAllocs()

		for b.Loop() {
			_, _ = withoutTimeout.Allow(ctx, "key")
		}
	})
}
