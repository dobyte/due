package file

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log/internal"
)

// newFileEntity builds a log entity for the file syncer tests.
func newFileEntity(now time.Time, message string) *internal.Entity {
	return &internal.Entity{
		Now:     now,
		Time:    now.Format("2006-01-02 15:04:05"),
		Level:   internal.LevelInfo,
		Message: message,
	}
}

// mustWriteFile writes data to path, failing the test on error.
func mustWriteFile(t *testing.T, path string, data string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write %s failed: %v", path, err)
	}
}

// TestSyncer_WriteImmediateFlush verifies immediate flushing and the closed state.
func TestSyncer_WriteImmediateFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(0), WithBufferSize(1024))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}

	if got, want := s.Name(), Name; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	if err := s.Write(newFileEntity(time.Now(), "hello immediate")); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if !strings.Contains(string(data), "hello immediate") {
		t.Errorf("log file = %q, want it to contain %q", data, "hello immediate")
	}

	if err := s.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
	if err := s.Write(newFileEntity(time.Now(), "after close")); err != errors.ErrSyncerClosed {
		t.Errorf("Write() after Close = %v, want %v", err, errors.ErrSyncerClosed)
	}
	if err := s.Close(); err != errors.ErrSyncerClosed {
		t.Errorf("second Close() = %v, want %v", err, errors.ErrSyncerClosed)
	}
}

// TestSyncer_NestedDir verifies that the syncer creates missing directories.
func TestSyncer_NestedDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	if err := s.Write(newFileEntity(time.Now(), "nested")); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("Stat() = %v, want the log file to exist", err)
	}
}

// TestSyncer_JsonFormat verifies the JSON formatter selection.
func TestSyncer_JsonFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFormat(FormatJson), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	if err := s.Write(newFileEntity(time.Now(), "json message")); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if !strings.Contains(string(data), `"msg"`) || !strings.Contains(string(data), "json message") {
		t.Errorf("log file = %q, want JSON containing %q", data, "json message")
	}
}

// TestSyncer_BatchedWrite verifies the channel path used when the flush lock is contended.
func TestSyncer_BatchedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(time.Hour))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}

	// Hold the flush lock so the write falls back to the channel path.
	s.mu.Lock()

	done := make(chan error, 1)
	go func() {
		done <- s.Write(newFileEntity(time.Now(), "batched message"))
	}()

	for i := 0; i < 1_000_000 && s.acc.Load() == 0; i++ {
		runtime.Gosched()
	}

	s.mu.Unlock()

	if err := <-done; err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if !strings.Contains(string(data), "batched message") {
		t.Errorf("log file = %q, want it to contain %q", data, "batched message")
	}
}

// TestSyncer_DoWriteClosing verifies both closing branches of doWrite.
func TestSyncer_DoWriteClosing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	t.Run("lock acquired while closing", func(t *testing.T) {
		s.closing.Store(true)

		e := s.pool.Get().(*entry)
		e.now = time.Now()

		if err := s.doWrite(e); err != errors.ErrSyncerClosed {
			t.Errorf("doWrite() = %v, want %v", err, errors.ErrSyncerClosed)
		}

		s.closing.Store(false)
	})

	t.Run("lock contended while closing", func(t *testing.T) {
		s.mu.Lock()
		s.closing.Store(true)

		e := s.pool.Get().(*entry)
		e.now = time.Now()

		if err := s.doWrite(e); err != errors.ErrSyncerClosed {
			t.Errorf("doWrite() = %v, want %v", err, errors.ErrSyncerClosed)
		}

		s.mu.Unlock()
		s.closing.Store(false)
	})
}

// TestSyncer_TryFlushToFileClosing verifies tryFlushToFile skips flushing when closing.
func TestSyncer_TryFlushToFileClosing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	s.closing.Store(true)
	s.tryFlushToFile()
	s.closing.Store(false)
}

// TestSyncer_FlushToWriterNoData verifies flushing with no buffered data and no open file.
func TestSyncer_FlushToWriterNoData(t *testing.T) {
	s := &Syncer{}
	s.opts = &options{}

	if err := s.flushToWriter(false); err != nil {
		t.Errorf("flushToWriter(false) = %v, want nil", err)
	}
}

// TestSyncer_FlushToWriterEmptyChannel verifies the default branch when the counter has no entry.
func TestSyncer_FlushToWriterEmptyChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(time.Hour))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	s.acc.Add(1)
	if err := s.flushToWriter(true); err != nil {
		t.Errorf("flushToWriter(true) = %v, want nil", err)
	}
	s.acc.Store(0)
}

// TestSyncer_RotateFileNil verifies rotateFile returns early without an open file.
func TestSyncer_RotateFileNil(t *testing.T) {
	s := &Syncer{}

	if err := s.rotateFile(); err != nil {
		t.Errorf("rotateFile() = %v, want nil", err)
	}
}

// TestSyncer_FlushToWriterDrain verifies draining the channel without auto flush.
func TestSyncer_FlushToWriterDrain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(time.Hour), WithBufferSize(1024))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}

	e := &entry{now: time.Now()}
	e.buf = s.formatter.Format(newFileEntity(time.Now(), "drained message"))
	s.acc.Add(1)
	s.chEntry <- e

	if err := s.flushToWriter(false); err != nil {
		t.Fatalf("flushToWriter(false) unexpected error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if !strings.Contains(string(data), "drained message") {
		t.Errorf("log file = %q, want it to contain %q", data, "drained message")
	}
}

// TestSyncer_FlushToFileError verifies the error path when the file cannot be opened.
func TestSyncer_FlushToFileError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	mustWriteFile(t, blocker, "not a directory")

	s := &Syncer{}
	s.opts = &options{path: filepath.Join(blocker, "due.log"), bufferSize: 1024}

	if err := s.flushToFile(&entry{now: time.Now()}); err == nil {
		t.Error("flushToFile() = nil, want an error")
	}
}

// TestSyncer_RotateByMaxSize verifies rotation driven by the maximum file size.
func TestSyncer_RotateByMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.log")

	s := NewSyncer(WithPath(path), WithMaxSize(16), WithBufferSize(8), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	if err := s.Write(newFileEntity(time.Now(), strings.Repeat("x", 64))); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "due.1.log")); err != nil {
		t.Errorf("rotated file missing: %v", err)
	}
}

// TestSyncer_RotateByTimeTag verifies rotation driven by the file tag.
func TestSyncer_RotateByTimeTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.log")

	s := NewSyncer(WithPath(path), WithRotate(RotateHour), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	if err := s.Write(newFileEntity(time.Now(), "first")); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	// Force a stale but recent tag so that the next write triggers a rotation without being
	// removed by the retention cleanup.
	staleTag := time.Now().Add(-time.Hour).Format("2006010215")
	s.setFileTag(staleTag)

	if err := s.Write(newFileEntity(time.Now(), "second")); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "due."+staleTag+".1.log")); err != nil {
		entries, _ := os.ReadDir(dir)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("rotated file missing: %v (dir: %v, tag=%q, version=%d)", err, names, s.getFileTag(), s.fileVersion)
	}
}

// TestSyncer_Compress verifies that a rotated file is compressed on close.
func TestSyncer_Compress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.log")

	s := NewSyncer(WithPath(path), WithMaxSize(16), WithCompress(true), WithBufferSize(8), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}

	if err := s.Write(newFileEntity(time.Now(), strings.Repeat("y", 64))); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	// Close waits for the compression goroutine.
	if err := s.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "due.1.gz")); err != nil {
		t.Errorf("compressed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "due.1.log")); !os.IsNotExist(err) {
		t.Errorf("source file still present: %v", err)
	}
}

// TestSyncer_CompressFile verifies compressFile directly for success and failure.
func TestSyncer_CompressFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.log")
	dst := filepath.Join(dir, "dst.gz")
	mustWriteFile(t, src, "compress me")

	s := &Syncer{}

	if err := s.compressFile(dst, src); err != nil {
		t.Fatalf("compressFile() unexpected error: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("compressed file missing: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source file still present: %v", err)
	}

	if err := s.compressFile(filepath.Join(dir, "missing.gz"), filepath.Join(dir, "missing.log")); err == nil {
		t.Error("compressFile() with a missing source = nil, want an error")
	}
}

// TestSyncer_DoRotateFileVersionCollision verifies the version probing used by doRotateFile.
func TestSyncer_DoRotateFileVersionCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.log")

	mustWriteFile(t, path, "active")
	mustWriteFile(t, filepath.Join(dir, "due.1.log"), "v1")
	mustWriteFile(t, filepath.Join(dir, "due.2.gz"), "v2")

	s := NewSyncer(WithPath(path), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	if err := s.doRotateFile("", 1); err != nil {
		t.Fatalf("doRotateFile() unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "due.3.log")); err != nil {
		t.Errorf("rotated file missing: %v", err)
	}
}

// TestSyncer_CleanExpiredFiles verifies removal of expired files and the skipped entries.
func TestSyncer_CleanExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.log")

	s := NewSyncer(WithPath(path), WithMaxAge(time.Hour), WithRotate(RotateDay), WithFlushInterval(0))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}
	defer s.Close()

	expired := []string{
		filepath.Join(dir, "due.19990101.1.log"),
		filepath.Join(dir, "due.19990101.log.gz"),
	}
	for _, p := range expired {
		mustWriteFile(t, p, "expired")
	}

	// These entries must be skipped by cleanExpiredFiles.
	mustWriteFile(t, filepath.Join(dir, "due.5.log"), "recent")
	mustWriteFile(t, filepath.Join(dir, "due.a.b.c.log"), "unparsable")
	mustWriteFile(t, filepath.Join(dir, "due.19990101.1.txt"), "wrong ext")
	mustWriteFile(t, filepath.Join(dir, "other.19990101.1.log"), "wrong prefix")
	mustWriteFile(t, filepath.Join(dir, "short"), "short")
	mustWriteFile(t, path, "active")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	s.cleanExpiredFiles()

	for _, p := range expired {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expired file %s still present: %v", p, err)
		}
	}
}

// TestSyncer_CleanExpiredFilesNoMaxAge verifies cleanExpiredFiles returns early without maxAge.
func TestSyncer_CleanExpiredFilesNoMaxAge(t *testing.T) {
	s := &Syncer{}
	s.fileName = "due"
	s.fileExt = ".log"
	s.opts = &options{maxAge: 0, path: filepath.Join(t.TempDir(), "due.log")}

	s.cleanExpiredFiles()
}

// TestSyncer_NewSyncerInvalidDir verifies NewSyncer fails when the log directory is a file.
func TestSyncer_NewSyncerInvalidDir(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	mustWriteFile(t, blocker, "not a directory")

	s := NewSyncer(WithPath(filepath.Join(blocker, "due.log")), WithMaxAge(time.Hour))
	if s != nil {
		t.Error("NewSyncer() = non-nil, want nil for an invalid directory")
	}
}

// TestSyncer_PathForms verifies the file name parsing for the supported path forms.
func TestSyncer_PathForms(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		wantName string
		wantExt  string
	}{
		{name: "no extension", file: "due", wantName: "due", wantExt: ""},
		{name: "single extension", file: "due.log", wantName: "due", wantExt: ".log"},
		{name: "multiple extensions", file: "due.log.old", wantName: "due.log", wantExt: ".old"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Syncer{}
			s.opts = &options{path: filepath.Join(t.TempDir(), tt.file), bufferSize: 1024}

			if err := s.init(); err != nil {
				t.Fatalf("init() unexpected error: %v", err)
			}

			if s.fileName != tt.wantName {
				t.Errorf("fileName = %q, want %q", s.fileName, tt.wantName)
			}
			if s.fileExt != tt.wantExt {
				t.Errorf("fileExt = %q, want %q", s.fileExt, tt.wantExt)
			}
			s.cancel()
		})
	}
}

// TestSyncer_ParseFileMark verifies parsing the file marks from existing files.
func TestSyncer_ParseFileMark(t *testing.T) {
	t.Run("version without tag", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "due.5.log"), "v5")

		s := NewSyncer(WithPath(filepath.Join(dir, "due.log")), WithFlushInterval(0))
		if s == nil {
			t.Fatal("NewSyncer() = nil, want non-nil")
		}
		defer s.Close()

		if got, want := s.fileVersion, int64(5); got != want {
			t.Errorf("fileVersion = %d, want %d", got, want)
		}
	})

	t.Run("tag and version reset", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "due.20240101.3.log"), "tagged")
		mustWriteFile(t, filepath.Join(dir, "due.20240101.4.gz"), "tagged gz")

		s := NewSyncer(WithPath(filepath.Join(dir, "due.log")), WithFlushInterval(0))
		if s == nil {
			t.Fatal("NewSyncer() = nil, want non-nil")
		}
		defer s.Close()

		if got := s.getFileTag(); got != "" {
			t.Errorf("fileTag = %q, want empty", got)
		}
		if got := s.fileVersion; got != 0 {
			t.Errorf("fileVersion = %d, want 0", got)
		}
	})

	t.Run("invalid and skipped files", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "due.x.log"), "bad version")
		mustWriteFile(t, filepath.Join(dir, "due.20240101.x.log"), "bad tagged version")
		mustWriteFile(t, filepath.Join(dir, "due.a.b.c.log"), "too many tags")
		mustWriteFile(t, filepath.Join(dir, "du.log"), "short name")
		mustWriteFile(t, filepath.Join(dir, "due.x.txt"), "wrong ext")
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}

		s := NewSyncer(WithPath(filepath.Join(dir, "due.log")), WithFlushInterval(0))
		if s == nil {
			t.Fatal("NewSyncer() = nil, want non-nil")
		}
		defer s.Close()

		if got := s.fileVersion; got != 0 {
			t.Errorf("fileVersion = %d, want 0", got)
		}
	})
}

// TestSyncer_TickFlushFile verifies the periodic flush goroutine.
func TestSyncer_TickFlushFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.log")

	s := NewSyncer(WithPath(path), WithFlushInterval(time.Millisecond))
	if s == nil {
		t.Fatal("NewSyncer() = nil, want non-nil")
	}

	if err := s.Write(newFileEntity(time.Now(), "tick flush")); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	// Give the ticker a chance to run before closing.
	time.Sleep(20 * time.Millisecond)

	if err := s.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
}

// TestSyncer_Helpers verifies the small tag and file name helpers.
func TestSyncer_Helpers(t *testing.T) {
	s := &Syncer{}
	s.fileName = "due"

	if got, want := s.makeFileName("", 1, ".log"), "due.1.log"; got != want {
		t.Errorf("makeFileName() = %q, want %q", got, want)
	}
	if got, want := s.makeFileName("202401", 2, ".log"), "due.202401.2.log"; got != want {
		t.Errorf("makeFileName() = %q, want %q", got, want)
	}

	if got := s.getFileTag(); got != "" {
		t.Errorf("getFileTag() = %q, want empty", got)
	}

	s.setFileTag("202401")
	if got, want := s.getFileTag(), "202401"; got != want {
		t.Errorf("getFileTag() = %q, want %q", got, want)
	}
}

// TestSyncer_FileModTime verifies reading the modification time from a directory entry.
func TestSyncer_FileModTime(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "due.log"), "x")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() unexpected error: %v", err)
	}

	s := &Syncer{}
	if got := s.fileModTime(entries[0]); got.IsZero() {
		t.Error("fileModTime() = zero, want a valid time")
	}
}

// TestSyncer_MakeFileTag verifies the tag format for every rotation rule.
func TestSyncer_MakeFileTag(t *testing.T) {
	now := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		rotate Rotate
		want   string
	}{
		{name: "none", rotate: RotateNone, want: ""},
		{name: "year", rotate: RotateYear, want: "2024"},
		{name: "month", rotate: RotateMonth, want: "202401"},
		{name: "week", rotate: RotateWeek, want: "202401"},
		{name: "day", rotate: RotateDay, want: "20240102"},
		{name: "hour", rotate: RotateHour, want: "2024010215"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Syncer{}
			s.opts = &options{rotate: tt.rotate}

			if got := s.makeFileTag(now); got != tt.want {
				t.Errorf("makeFileTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSyncer_ParseFileTagTime verifies parsing tags for every rotation rule.
func TestSyncer_ParseFileTagTime(t *testing.T) {
	tests := []struct {
		name   string
		rotate Rotate
		tag    string
		wantOK bool
	}{
		{name: "year", rotate: RotateYear, tag: "2024", wantOK: true},
		{name: "year invalid", rotate: RotateYear, tag: "abcd", wantOK: false},
		{name: "month", rotate: RotateMonth, tag: "202401", wantOK: true},
		{name: "month invalid", rotate: RotateMonth, tag: "abcd", wantOK: false},
		{name: "week", rotate: RotateWeek, tag: "202401", wantOK: true},
		{name: "week out of range", rotate: RotateWeek, tag: "202400", wantOK: false},
		{name: "week invalid", rotate: RotateWeek, tag: "abcd01", wantOK: false},
		{name: "day", rotate: RotateDay, tag: "20240102", wantOK: true},
		{name: "day invalid", rotate: RotateDay, tag: "abcd", wantOK: false},
		{name: "hour", rotate: RotateHour, tag: "2024010215", wantOK: true},
		{name: "hour invalid", rotate: RotateHour, tag: "abcd", wantOK: false},
		{name: "none", rotate: RotateNone, tag: "20240102", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Syncer{}
			s.opts = &options{rotate: tt.rotate}

			_, ok := s.parseFileTagTime(tt.tag)
			if ok != tt.wantOK {
				t.Errorf("parseFileTagTime(%q) = %v, want %v", tt.tag, ok, tt.wantOK)
			}
		})
	}
}

// TestSyncer_FilterFileMark verifies every branch of filterFileMark.
func TestSyncer_FilterFileMark(t *testing.T) {
	s := &Syncer{}
	s.setFileTag("20240101")
	s.fileVersion = 3

	s.filterFileMark("20230101", 5)
	if got, want := s.getFileTag(), "20240101"; got != want {
		t.Errorf("fileTag after older tag = %q, want %q", got, want)
	}
	if got, want := s.fileVersion, int64(3); got != want {
		t.Errorf("fileVersion after older tag = %d, want %d", got, want)
	}

	s.filterFileMark("20240101", 2)
	if got, want := s.fileVersion, int64(3); got != want {
		t.Errorf("fileVersion after smaller version = %d, want %d", got, want)
	}

	s.filterFileMark("20240101", 4)
	if got, want := s.fileVersion, int64(4); got != want {
		t.Errorf("fileVersion after larger version = %d, want %d", got, want)
	}

	s.filterFileMark("20250101", 1)
	if got, want := s.getFileTag(), "20250101"; got != want {
		t.Errorf("fileTag after newer tag = %q, want %q", got, want)
	}
	if got, want := s.fileVersion, int64(1); got != want {
		t.Errorf("fileVersion after newer tag = %d, want %d", got, want)
	}
}
