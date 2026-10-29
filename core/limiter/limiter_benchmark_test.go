package limiter_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/limiter"
)

func BenchmarkLimiter_Allow(b *testing.B) {
	l := limiter.NewLimiter(1e9, 1e9)

	b.ReportAllocs()

	for b.Loop() {
		l.Allow()
	}
}

func BenchmarkLimiter_Take(b *testing.B) {
	l := limiter.NewLimiter(1e9, 1e9)

	b.ReportAllocs()

	for b.Loop() {
		l.Take(1)
	}
}

func BenchmarkLimiter_AllowParallel(b *testing.B) {
	l := limiter.NewLimiter(1e9, 1e9)

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Allow()
		}
	})
}
