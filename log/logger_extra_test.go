package log_test

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/log"
)

// capturedEntity is a snapshot of a written entity, kept independent of the logger pool.
type capturedEntity struct {
	level   log.Level
	message string
	caller  string
	frames  int
}

// captureSyncer is a Syncer implementation that records the written entities in memory.
type captureSyncer struct {
	name string
	mu   sync.Mutex
	got  []capturedEntity
	done int
}

// Name returns the syncer name.
func (c *captureSyncer) Name() string {
	return c.name
}

// Write records the given entity.
func (c *captureSyncer) Write(entity *log.Entity) error {
	c.mu.Lock()
	c.got = append(c.got, capturedEntity{
		level:   entity.Level,
		message: entity.Message,
		caller:  entity.Caller,
		frames:  len(entity.Frames),
	})
	c.mu.Unlock()

	return nil
}

// Close marks the syncer as closed.
func (c *captureSyncer) Close() error {
	c.mu.Lock()
	c.done++
	c.mu.Unlock()

	return nil
}

// snapshot returns a copy of the recorded entities.
func (c *captureSyncer) snapshot() []capturedEntity {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]capturedEntity, len(c.got))
	copy(out, c.got)

	return out
}

// newCaptureLogger builds a logger wired to a capture syncer.
func newCaptureLogger(opts ...log.Option) (log.Logger, *captureSyncer) {
	syncer := &captureSyncer{name: "capture"}

	base := []log.Option{
		log.WithSyncers(syncer),
		log.WithTerminals(log.Terminal("capture")),
	}

	return log.NewLogger(append(base, opts...)...), syncer
}

// TestLogger_AllLevels verifies that every convenience method emits a record.
func TestLogger_AllLevels(t *testing.T) {
	logger, syncer := newCaptureLogger(log.WithLevel(log.LevelNone))

	logger.Debug("debug")
	logger.Debugf("debug %d", 1)
	logger.Info("info")
	logger.Infof("info %d", 1)
	logger.Warn("warn")
	logger.Warnf("warn %d", 1)
	logger.Error("error")
	logger.Errorf("error %d", 1)
	logger.Panic("panic")
	logger.Panicf("panic %d", 1)
	logger.Print(log.LevelInfo, "print")
	logger.Printf(log.LevelInfo, "print %d", 1)

	got := syncer.snapshot()
	if len(got) != 12 {
		t.Fatalf("captured %d records, want 12", len(got))
	}

	wantLevels := []log.Level{
		log.LevelDebug, log.LevelDebug,
		log.LevelInfo, log.LevelInfo,
		log.LevelWarn, log.LevelWarn,
		log.LevelError, log.LevelError,
		log.LevelPanic, log.LevelPanic,
		log.LevelInfo, log.LevelInfo,
	}
	for i, want := range wantLevels {
		if got[i].level != want {
			t.Errorf("record %d level = %q, want %q", i, got[i].level, want)
		}
	}
}

// TestLogger_LevelFilter verifies that records below the configured level are dropped.
func TestLogger_LevelFilter(t *testing.T) {
	logger, syncer := newCaptureLogger(log.WithLevel(log.LevelError))

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	got := syncer.snapshot()
	if len(got) != 1 {
		t.Fatalf("captured %d records, want 1", len(got))
	}
	if got[0].level != log.LevelError {
		t.Errorf("level = %q, want %q", got[0].level, log.LevelError)
	}
}

// TestLogger_TerminalLevelFilter verifies per-terminal level filtering.
func TestLogger_TerminalLevelFilter(t *testing.T) {
	syncer := &captureSyncer{name: "capture"}

	logger := log.NewLogger(
		log.WithSyncers(syncer),
		log.WithTerminals(map[log.Terminal][]log.Level{
			log.Terminal("capture"): {log.LevelError},
		}),
		log.WithLevel(log.LevelNone),
	)

	logger.Debug("debug")
	logger.Error("error")

	got := syncer.snapshot()
	if len(got) != 1 {
		t.Fatalf("captured %d records, want 1", len(got))
	}
	if got[0].level != log.LevelError {
		t.Errorf("level = %q, want %q", got[0].level, log.LevelError)
	}
}

// TestLogger_NoTerminals verifies that logging without terminals is a no-op.
func TestLogger_NoTerminals(t *testing.T) {
	logger := log.NewLogger(log.WithTerminals([]log.Terminal{}))
	logger.Info("nothing to write")

	if err := logger.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}

	unknown := log.NewLogger(log.WithTerminals(log.Terminal("unknown")))
	unknown.Info("nothing to write")

	if err := unknown.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// TestLogger_BuiltinTerminals verifies the console and file terminal resolution.
func TestLogger_BuiltinTerminals(t *testing.T) {
	logger := log.NewLogger(
		log.WithTerminals(log.TerminalConsole, log.TerminalFile),
		log.WithLevel(log.LevelError),
	)

	if err := logger.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// TestLogger_SlogHandler verifies the slog.Handler implementation.
func TestLogger_SlogHandler(t *testing.T) {
	ctx := context.Background()

	logger, syncer := newCaptureLogger(log.WithLevel(log.LevelDebug))

	if !logger.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled(LevelInfo) = false, want true")
	}
	if logger.Enabled(ctx, slog.Level(12)) {
		t.Error("Enabled(Level(12)) = true, want false")
	}

	if err := logger.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelDebug, "debug", 0)); err != nil {
		t.Errorf("Handle() = %v, want nil", err)
	}
	if err := logger.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelInfo, "info", 0)); err != nil {
		t.Errorf("Handle() = %v, want nil", err)
	}
	if err := logger.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelWarn, "warn", 0)); err != nil {
		t.Errorf("Handle() = %v, want nil", err)
	}

	var pcs [1]uintptr
	runtime.Callers(1, pcs[:])
	if err := logger.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelError, "error", pcs[0])); err != nil {
		t.Errorf("Handle() = %v, want nil", err)
	}

	if logger.WithAttrs([]slog.Attr{slog.String("key", "value")}) == nil {
		t.Error("WithAttrs() = nil, want the logger")
	}
	if logger.WithGroup("group") == nil {
		t.Error("WithGroup() = nil, want the logger")
	}

	got := syncer.snapshot()
	if len(got) != 4 {
		t.Fatalf("captured %d records, want 4", len(got))
	}
	for i, want := range []log.Level{log.LevelDebug, log.LevelInfo, log.LevelWarn, log.LevelError} {
		if got[i].level != want {
			t.Errorf("record %d level = %q, want %q", i, got[i].level, want)
		}
	}
	if got[3].caller == "" || got[3].frames == 0 {
		t.Errorf("error record caller = %q, frames = %d, want a caller and stack", got[3].caller, got[3].frames)
	}

	unknownLogger, unknownSyncer := newCaptureLogger(log.WithLevel(log.LevelNone))
	if err := unknownLogger.Handle(ctx, slog.NewRecord(time.Now(), slog.Level(12), "unknown", 0)); err != nil {
		t.Errorf("Handle() = %v, want nil", err)
	}
	if got := unknownSyncer.snapshot(); len(got) != 1 || got[0].level != log.Level("") {
		t.Errorf("unknown level captured = %v, want a single empty-level record", got)
	}
}

// TestLogger_Stack verifies stack emission and the call path options.
func TestLogger_Stack(t *testing.T) {
	logger, syncer := newCaptureLogger(
		log.WithLevel(log.LevelNone),
		log.WithStackLevel(log.LevelError),
		log.WithCallFullPath(true),
		log.WithCallSkip(0),
	)

	logger.Info("info")
	logger.Error("error")

	got := syncer.snapshot()
	if len(got) != 2 {
		t.Fatalf("captured %d records, want 2", len(got))
	}

	if got[0].caller == "" {
		t.Error("info record caller is empty, want a caller")
	}
	if got[0].frames != 0 {
		t.Errorf("info record frames = %d, want 0", got[0].frames)
	}
	if got[1].caller == "" || got[1].frames == 0 {
		t.Errorf("error record caller = %q, frames = %d, want a caller and stack", got[1].caller, got[1].frames)
	}
}

// TestLogger_MessageFormatting verifies message assembly and trailing newline trimming.
func TestLogger_MessageFormatting(t *testing.T) {
	logger, syncer := newCaptureLogger(log.WithLevel(log.LevelNone))

	logger.Info("a", 1, "b")
	logger.Info("trailing\n")

	got := syncer.snapshot()
	if len(got) != 2 {
		t.Fatalf("captured %d records, want 2", len(got))
	}
	if got[0].message != "a 1 b" {
		t.Errorf("message = %q, want %q", got[0].message, "a 1 b")
	}
	if got[1].message != "trailing" {
		t.Errorf("message = %q, want %q", got[1].message, "trailing")
	}
}

// TestLogger_Close verifies that Close closes every terminal syncer.
func TestLogger_Close(t *testing.T) {
	logger, syncer := newCaptureLogger()

	if err := logger.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("second Close() = %v, want nil", err)
	}

	syncer.mu.Lock()
	done := syncer.done
	syncer.mu.Unlock()

	if done < 1 {
		t.Errorf("syncer closed %d times, want at least 1", done)
	}
}

// TestLogger_Options verifies that every option is applied.
func TestLogger_Options(t *testing.T) {
	syncer := &captureSyncer{name: "capture"}

	logger := log.NewLogger(
		log.WithSyncers(syncer),
		log.WithLevel(log.LevelDebug),
		log.WithStackLevel(log.LevelWarn),
		log.WithTimeFormat("2006-01-02"),
		log.WithCallSkip(1),
		log.WithCallFullPath(true),
		log.WithTerminals([]log.Terminal{log.Terminal("capture")}),
	)

	logger.Warn("warn message")

	got := syncer.snapshot()
	if len(got) != 1 {
		t.Fatalf("captured %d records, want 1", len(got))
	}
	if got[0].message != "warn message" {
		t.Errorf("message = %q, want %q", got[0].message, "warn message")
	}
	if got[0].caller == "" || got[0].frames == 0 {
		t.Errorf("caller = %q, frames = %d, want a caller and stack", got[0].caller, got[0].frames)
	}
}

// TestLogger_GlobalFunctions verifies the package-level logging helpers.
func TestLogger_GlobalFunctions(t *testing.T) {
	old := log.GetLogger()
	defer log.SetLogger(old)

	syncer := &captureSyncer{name: "capture"}
	logger := log.NewLogger(
		log.WithSyncers(syncer),
		log.WithTerminals(log.Terminal("capture")),
		log.WithLevel(log.LevelNone),
	)
	log.SetLogger(logger)

	log.Print(log.LevelInfo, "print")
	log.Printf(log.LevelInfo, "printf %d", 1)
	log.Debug("debug")
	log.Debugf("debugf %d", 1)
	log.Info("info")
	log.Infof("infof %d", 1)
	log.Warn("warn")
	log.Warnf("warnf %d", 1)
	log.Error("error")
	log.Errorf("errorf %d", 1)
	log.Panic("panic")
	log.Panicf("panicf %d", 1)

	if got := log.GetLogger(); got != logger {
		t.Error("GetLogger() did not return the logger set by SetLogger()")
	}

	if got := len(syncer.snapshot()); got != 12 {
		t.Errorf("captured %d records, want 12", got)
	}

	log.Close()
	log.SetLogger(nil)
}
