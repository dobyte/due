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

// benchFlushIntervalCases 用于对比批量刷写（Batch）与每条立即刷盘（Immediate）的性能差异。
var benchFlushIntervalCases = []struct {
	name  string
	value time.Duration
}{
	{name: "Batch", value: time.Second},
	{name: "Immediate", value: 0},
}

// newBenchEntity 构造基准测试使用的日志实体。
// 该实体在压测期间只读，可被多个 goroutine 安全复用。
func newBenchEntity(message string) *internal.Entity {
	return &internal.Entity{
		Now:     time.Now(),
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Level:   internal.LevelInfo,
		Message: message,
	}
}

// BenchmarkSyncerSerial 串行场景：单 goroutine 顺序写入单个 Syncer 实例，
// 作为基准线，用于与并发、并行场景对比，并对比批量刷写与每条立即刷盘的差异。
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

					// 预热：触发文件懒打开，避免首次打开的开销计入计时
					if err := s.Write(entity); err != nil {
						b.Fatal(err)
					}

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						// 基准测试专注吞吐，错误由单元测试覆盖
						_ = s.Write(entity)
					}
				})
			}
		})
	}
}

// BenchmarkSyncerConcurrent 并发场景：单个 Syncer 实例，多个 goroutine 同时写入。
// 用于评估多 goroutine 竞争下的锁、channel 缓冲与批量刷写表现，并对比批量刷写与每条立即刷盘的差异。
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

					// 预热：触发文件懒打开，避免首次打开的开销计入计时
					if err := s.Write(entity); err != nil {
						b.Fatal(err)
					}

					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							// 基准测试专注吞吐，错误由单元测试覆盖
							_ = s.Write(entity)
						}
					})
				})
			}
		})
	}
}

// BenchmarkSyncerParallel 并行场景：预创建 GOMAXPROCS 个独立 Syncer 实例，
// 每个 goroutine 独占一个实例与文件、无共享状态，用于评估多核下的并行扩展性，
// 并对比批量刷写与每条立即刷盘的差异。
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

					// 预创建实例池，排除实例创建开销，使计时只包含 Write
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
							// 基准测试专注吞吐，错误由单元测试覆盖
							_ = s.Write(entity)
						}
					})
				})
			}
		})
	}
}
