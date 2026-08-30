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

// BenchmarkSyncerConcurrent 并发场景：单个 Syncer 实例，多个 goroutine 同时写入。
// 用于评估多 goroutine 竞争下的锁、channel 缓冲与批量刷写表现。
func BenchmarkSyncerConcurrent(b *testing.B) {
	for _, tc := range []struct {
		name    string
		message string
	}{
		{name: "Small", message: benchSmallMessage},
		{name: "Large", message: benchLargeMessage},
	} {
		b.Run(tc.name, func(b *testing.B) {
			s := NewSyncer(WithPath(filepath.Join(b.TempDir(), "due.log")))
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
}

// BenchmarkSyncerParallel 并行场景：预创建 GOMAXPROCS 个独立 Syncer 实例，
// 每个 goroutine 独占一个实例与文件、无共享状态，用于评估多核下的并行扩展性。
func BenchmarkSyncerParallel(b *testing.B) {
	for _, tc := range []struct {
		name    string
		message string
	}{
		{name: "Small", message: benchSmallMessage},
		{name: "Large", message: benchLargeMessage},
	} {
		b.Run(tc.name, func(b *testing.B) {
			n := runtime.GOMAXPROCS(0)

			// 预创建实例池，排除实例创建开销，使计时只包含 Write
			ch := make(chan *Syncer, n)
			for i := 0; i < n; i++ {
				s := NewSyncer(WithPath(filepath.Join(b.TempDir(), "due.log")))
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
}
