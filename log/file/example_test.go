package file_test

import (
	"os"
	"path/filepath"
	"time"

	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/log/file"
)

// ExampleNewSyncer 高性能模式：批量刷写 + 定时刷盘（默认语义）。
// 吞吐最高，进程崩溃最多丢失最近一个 flushInterval 内的日志。
func ExampleNewSyncer() {
	dir, _ := os.MkdirTemp("", "due-log-example")
	defer os.RemoveAll(dir)

	syncer := file.NewSyncer(
		file.WithPath(filepath.Join(dir, "due.log")),
		file.WithFormat(file.FormatJson),
		file.WithFlushInterval(time.Second), // >0：批量写 + 每 1s 定时刷盘
		file.WithBufferSize(32<<10),         // 32KB 缓冲
		file.WithMaxSize(500<<20),           // 单个文件 500MB
		file.WithMaxAge(7*24*time.Hour),     // 保留 7 天
		file.WithRotate(file.RotateDay),     // 按天轮转
		file.WithCompress(true),             // 压缩轮转文件
	)
	defer syncer.Close()

	entity := &log.Entity{
		Now:     time.Now(),
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Level:   log.LevelInfo,
		Message: "hello due-framework",
	}

	_ = syncer.Write(entity)
}

// ExampleNewSyncer_immediateFlush 高可靠模式：每条日志立即刷盘。
// 进程崩溃最多丢失正在写入的那一条，代价是吞吐下降。
func ExampleNewSyncer_immediateFlush() {
	dir, _ := os.MkdirTemp("", "due-log-example")
	defer os.RemoveAll(dir)

	syncer := file.NewSyncer(
		file.WithPath(filepath.Join(dir, "due.log")),
		file.WithFlushInterval(0), // <=0：每条日志立即刷盘
	)
	defer syncer.Close()

	entity := &log.Entity{
		Now:     time.Now(),
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Level:   log.LevelError,
		Message: "something went wrong",
	}

	_ = syncer.Write(entity)
}
