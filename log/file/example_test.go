package file_test

import (
	"os"
	"path/filepath"
	"time"

	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/log/file"
)

// ExampleNewSyncer uses the high-performance mode: batched writes with periodic flushing (the
// default semantics). It offers the highest throughput; a process crash loses at most the log
// records written within the last flushInterval.
func ExampleNewSyncer() {
	dir, _ := os.MkdirTemp("", "due-log-example")
	defer os.RemoveAll(dir)

	syncer := file.NewSyncer(
		file.WithPath(filepath.Join(dir, "due.log")),
		file.WithFormat(file.FormatJson),
		file.WithFlushInterval(time.Second), // >0: batched writes flushed every second
		file.WithBufferSize(32<<10),         // 32KB buffer
		file.WithMaxSize(500<<20),           // 500MB per file
		file.WithMaxAge(7*24*time.Hour),     // retain for 7 days
		file.WithRotate(file.RotateDay),     // rotate daily
		file.WithCompress(true),             // compress rotated files
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

// ExampleNewSyncer_immediateFlush uses the high-reliability mode: every log record is flushed
// immediately. A process crash loses at most the record being written, at the cost of lower
// throughput.
func ExampleNewSyncer_immediateFlush() {
	dir, _ := os.MkdirTemp("", "due-log-example")
	defer os.RemoveAll(dir)

	syncer := file.NewSyncer(
		file.WithPath(filepath.Join(dir, "due.log")),
		file.WithFlushInterval(0), // <=0: flush every log record immediately
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
