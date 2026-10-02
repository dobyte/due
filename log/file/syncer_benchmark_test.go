package file

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/log/internal"
)

var (
	benchSmallMessage = "a short log message for benchmark"
	benchLargeMessage = strings.Repeat("x", 1024)
)

// benchFlushIntervalCases compares the performance of batched writes (Batch) with immediate flush (Immediate).
var benchFlushIntervalCases = []struct {
	name  string
	value time.Duration
}{
	{name: "Batch", value: time.Second},
	{name: "Immediate", value: 0},
}

// newBenchEntity builds the log entity used by the benchmarks.
// The entity is read-only during the benchmark and can be safely shared by multiple goroutines.
func newBenchEntity(message string) *internal.Entity {
	return &internal.Entity{
		Now:     time.Now(),
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Level:   internal.LevelInfo,
		Message: message,
	}
}

// BenchmarkSyncerSerial benchmarks the serial case: a single goroutine writes to a single Syncer
// sequentially. It serves as the baseline for the concurrent and parallel cases and compares
// batched writes with immediate flushing.
func BenchmarkSyncerSerial(b *testing.B) {
	for _, tc := range []struct {
		name    string
		message string
	}{
		{name: "Small", message: benchSmallMessage},
		{name: "Large", message: benchLargeMessage},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, fc := range benchFlushIntervalCases {
				b.Run(fc.name, func(b *testing.B) {
					s := NewSyncer(WithPath(filepath.Join(b.TempDir(), "due.log")), WithFlushInterval(fc.value))
					defer s.Close()

					entity := newBenchEntity(tc.message)

					// Warm up: trigger the lazy file open so its cost is not counted in the timing.
					if err := s.Write(entity); err != nil {
						b.Fatal(err)
					}

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						// The benchmark focuses on throughput; errors are covered by unit tests.
						_ = s.Write(entity)
					}
				})
			}
		})
	}
}

// BenchmarkSyncerConcurrent benchmarks the concurrent case: multiple goroutines write to a single
// Syncer at the same time. It evaluates the lock, channel buffering and batched flushing behavior
// under contention and compares batched writes with immediate flushing.
func BenchmarkSyncerConcurrent(b *testing.B) {
	for _, tc := range []struct {
		name    string
		message string
	}{
		{name: "Small", message: benchSmallMessage},
		{name: "Large", message: benchLargeMessage},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, fc := range benchFlushIntervalCases {
				b.Run(fc.name, func(b *testing.B) {
					s := NewSyncer(WithPath(filepath.Join(b.TempDir(), "due.log")), WithFlushInterval(fc.value))
					defer s.Close()

					entity := newBenchEntity(tc.message)

					// Warm up: trigger the lazy file open so its cost is not counted in the timing.
					if err := s.Write(entity); err != nil {
						b.Fatal(err)
					}

					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							// The benchmark focuses on throughput; errors are covered by unit tests.
							_ = s.Write(entity)
						}
					})
				})
			}
		})
	}
}

// BenchmarkSyncerParallel benchmarks the parallel case: GOMAXPROCS independent Syncer instances are
// pre-created, and each goroutine owns one instance and file with no shared state. It evaluates the
// parallel scalability across cores and compares batched writes with immediate flushing.
func BenchmarkSyncerParallel(b *testing.B) {
	for _, tc := range []struct {
		name    string
		message string
	}{
		{name: "Small", message: benchSmallMessage},
		{name: "Large", message: benchLargeMessage},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, fc := range benchFlushIntervalCases {
				b.Run(fc.name, func(b *testing.B) {
					n := runtime.GOMAXPROCS(0)

					// Pre-create a pool of instances to exclude creation cost so that only Write is timed.
					ch := make(chan *Syncer, n)
					for i := 0; i < n; i++ {
						s := NewSyncer(WithPath(filepath.Join(b.TempDir(), "due.log")), WithFlushInterval(fc.value))
						ch <- s
						defer s.Close()
					}

					entity := newBenchEntity(tc.message)

					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						s := <-ch
						for pb.Next() {
							// The benchmark focuses on throughput; errors are covered by unit tests.
							_ = s.Write(entity)
						}
					})
				})
			}
		})
	}
}
