package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/dobyte/due/v2/core/stack"
	"github.com/dobyte/due/v2/log/console"
	"github.com/dobyte/due/v2/log/file"
	"github.com/dobyte/due/v2/log/internal"
	"github.com/dobyte/due/v2/utils/xtime"
	"golang.org/x/sync/errgroup"
)

// Logger is a logging interface that extends [slog.Handler].
type Logger interface {
	slog.Handler
	// Print writes a log record without stack information.
	Print(level Level, a ...any)
	// Printf writes a formatted log record without stack information.
	Printf(level Level, format string, a ...any)
	// Debug writes a debug-level log record.
	Debug(a ...any)
	// Debugf writes a formatted debug-level log record.
	Debugf(format string, a ...any)
	// Info writes an info-level log record.
	Info(a ...any)
	// Infof writes a formatted info-level log record.
	Infof(format string, a ...any)
	// Warn writes a warn-level log record.
	Warn(a ...any)
	// Warnf writes a formatted warn-level log record.
	Warnf(format string, a ...any)
	// Error writes an error-level log record.
	Error(a ...any)
	// Errorf writes a formatted error-level log record.
	Errorf(format string, a ...any)
	// Fatal writes a fatal-level log record.
	Fatal(a ...any)
	// Fatalf writes a formatted fatal-level log record.
	Fatalf(format string, a ...any)
	// Panic writes a panic-level log record.
	Panic(a ...any)
	// Panicf writes a formatted panic-level log record.
	Panicf(format string, a ...any)
	// Close closes the logger.
	Close() error
}

type terminal struct {
	syncer Syncer
	levels map[Level]bool
}

var _ slog.Handler = (*defaultLogger)(nil)

type defaultLogger struct {
	opts      *options
	pool      *sync.Pool
	terminals []*terminal
}

// NewLogger returns a new logger configured with the given options.
func NewLogger(opts ...Option) *defaultLogger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	l := &defaultLogger{}
	l.opts = o
	l.pool = &sync.Pool{New: func() any { return &Entity{} }}

	syncers := make(map[string]Syncer, len(l.opts.syncers))
	for _, syncer := range l.opts.syncers {
		if syncer != nil {
			syncers[syncer.Name()] = syncer
		}
	}

	switch v := l.opts.terminals.(type) {
	case []Terminal:
		for _, name := range v {
			syncer, ok := syncers[string(name)]
			if !ok {
				switch name {
				case TerminalConsole:
					syncer = console.NewSyncer()
				case TerminalFile:
					syncer = file.NewSyncer()
				}
			}

			if syncer == nil {
				continue
			}

			l.terminals = append(l.terminals, &terminal{
				syncer: syncer,
			})
		}
	case map[Terminal][]Level:
		for name, levels := range v {
			syncer, ok := syncers[string(name)]
			if !ok {
				switch name {
				case TerminalConsole:
					syncer = console.NewSyncer()
				case TerminalFile:
					syncer = file.NewSyncer()
				}
			}

			if syncer == nil {
				continue
			}

			t := &terminal{
				syncer: syncer,
				levels: make(map[Level]bool, len(levels)),
			}

			for _, level := range levels {
				t.levels[level] = true
			}

			l.terminals = append(l.terminals, t)
		}
	}

	return l
}

// Print writes a log record without stack information.
func (l *defaultLogger) Print(level Level, a ...any) {
	l.print(level, false, a...)
}

// Printf writes a formatted log record without stack information.
func (l *defaultLogger) Printf(level Level, format string, a ...any) {
	l.print(level, false, fmt.Sprintf(format, a...))
}

// Debug writes a debug-level log record.
func (l *defaultLogger) Debug(a ...any) {
	l.print(LevelDebug, true, a...)
}

// Debugf writes a formatted debug-level log record.
func (l *defaultLogger) Debugf(format string, a ...any) {
	l.print(LevelDebug, true, fmt.Sprintf(format, a...))
}

// Info writes an info-level log record.
func (l *defaultLogger) Info(a ...any) {
	l.print(LevelInfo, true, a...)
}

// Infof writes a formatted info-level log record.
func (l *defaultLogger) Infof(format string, a ...any) {
	l.print(LevelInfo, true, fmt.Sprintf(format, a...))
}

// Warn writes a warn-level log record.
func (l *defaultLogger) Warn(a ...any) {
	l.print(LevelWarn, true, a...)
}

// Warnf writes a formatted warn-level log record.
func (l *defaultLogger) Warnf(format string, a ...any) {
	l.print(LevelWarn, true, fmt.Sprintf(format, a...))
}

// Error writes an error-level log record.
func (l *defaultLogger) Error(a ...any) {
	l.print(LevelError, true, a...)
}

// Errorf writes a formatted error-level log record.
func (l *defaultLogger) Errorf(format string, a ...any) {
	l.print(LevelError, true, fmt.Sprintf(format, a...))
}

// Fatal writes a fatal-level log record.
func (l *defaultLogger) Fatal(a ...any) {
	l.print(LevelFatal, true, a...)
	os.Exit(1)
}

// Fatalf writes a formatted fatal-level log record.
func (l *defaultLogger) Fatalf(format string, a ...any) {
	l.print(LevelFatal, true, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// Panic writes a panic-level log record.
func (l *defaultLogger) Panic(a ...any) {
	l.print(LevelPanic, true, a...)
}

// Panicf writes a formatted panic-level log record.
func (l *defaultLogger) Panicf(format string, a ...any) {
	l.print(LevelPanic, true, fmt.Sprintf(format, a...))
}

// Close closes the logger.
func (l *defaultLogger) Close() error {
	eg, _ := errgroup.WithContext(context.Background())

	for i := range l.terminals {
		syncer := l.terminals[i].syncer

		eg.Go(func() error {
			return syncer.Close()
		})
	}

	return eg.Wait()
}

// print writes a log record at the given level, optionally including stack information.
func (l *defaultLogger) print(level Level, isOutStack bool, a ...any) {
	if len(l.terminals) == 0 {
		return
	}

	if level.Priority() < l.opts.level.Priority() {
		return
	}

	var entity *Entity

	for i := range l.terminals {
		t := l.terminals[i]

		if len(t.levels) > 0 && !t.levels[level] {
			continue
		}

		if entity == nil {
			entity = l.makeEntity(level, isOutStack, a...)
		}

		t.syncer.Write(entity)
	}

	if entity != nil {
		l.releaseEntity(entity)
	}
}

// releaseEntity releases the entity back to the pool.
func (l *defaultLogger) releaseEntity(e *Entity) {
	e.Time = ""
	e.Level = LevelNone
	e.Message = ""
	e.Caller = ""
	e.Frames = nil

	l.pool.Put(e)
}

// makeEntity builds a log entity.
func (l *defaultLogger) makeEntity(level Level, isOutStack bool, a ...any) *Entity {
	e := l.pool.Get().(*Entity)
	e.Now = xtime.Now()
	e.Time = e.Now.Format(l.opts.timeFormat)
	e.Level = level
	e.Message = l.makeMessage(a...)

	if isOutStack && l.opts.stackLevel != "" && l.opts.stackLevel != LevelNone && level.Priority() >= l.opts.stackLevel.Priority() {
		e.Caller, e.Frames = l.makeStack(stack.Full)
	} else {
		e.Caller, e.Frames = l.makeStack(stack.First)
	}

	return e
}

// makeMessage builds the log message from the given arguments.
func (l *defaultLogger) makeMessage(a ...any) (message string) {
	for i, v := range a {
		if i == len(a)-1 {
			message += internal.String(v)
		} else {
			message += internal.String(v) + " "
		}
	}

	message = strings.TrimSuffix(message, "\n")

	return
}

// makeStack builds the caller and stack frames at the given depth.
func (l *defaultLogger) makeStack(depth stack.Depth) (string, []runtime.Frame) {
	st := stack.Callers(3+l.opts.callSkip, depth)
	defer st.Free()

	var (
		caller string
		frames = st.Frames()
	)

	if len(frames) > 0 {
		file := frames[0].File
		line := frames[0].Line

		if !l.opts.callFullPath {
			_, file = filepath.Split(file)
		}

		caller = file + ":" + strconv.Itoa(line)
	}

	if depth == stack.First {
		return caller, nil
	} else {
		return caller, frames
	}
}

// Enabled reports whether the logger emits records of the given level.
func (l *defaultLogger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.convLevel(level).Priority() >= l.opts.level.Priority()
}

// Handle processes a slog record.
func (l *defaultLogger) Handle(ctx context.Context, record slog.Record) error {
	var (
		level  = l.convLevel(record.Level)
		entity *Entity
	)

	for i := range l.terminals {
		t := l.terminals[i]

		if len(t.levels) > 0 && !t.levels[level] {
			continue
		}

		if entity == nil {
			entity = l.convEntity(record)
		}

		t.syncer.Write(entity)
	}

	if entity != nil {
		l.releaseEntity(entity)
	}

	return nil
}

func (l *defaultLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return l
}

func (l *defaultLogger) WithGroup(name string) slog.Handler {
	return l
}

// convEntity converts a slog record into a log entity.
func (l *defaultLogger) convEntity(record slog.Record) *Entity {
	entity := l.pool.Get().(*Entity)
	entity.Now = record.Time
	entity.Time = record.Time.Format(l.opts.timeFormat)
	entity.Level = l.convLevel(record.Level)
	entity.Message = record.Message

	if l.opts.stackLevel != "" && l.opts.stackLevel != LevelNone && entity.Level.Priority() >= l.opts.stackLevel.Priority() {
		entity.Caller, entity.Frames = l.convStack(record.PC, stack.Full)
	} else {
		entity.Caller, entity.Frames = l.makeStack(stack.First)
	}

	return entity
}

// convStack converts the program counter into a caller and stack frames.
func (l *defaultLogger) convStack(pc uintptr, depth stack.Depth) (string, []runtime.Frame) {
	st := stack.CallersFromPC(pc, stack.Full)
	defer st.Free()

	var (
		caller string
		frames = st.Frames()
	)

	if len(frames) > 0 {
		file := frames[0].File
		line := frames[0].Line

		if !l.opts.callFullPath {
			_, file = filepath.Split(file)
		}

		caller = file + ":" + strconv.Itoa(line)
	}

	if depth == stack.First {
		return caller, nil
	} else {
		return caller, frames
	}
}

// convLevel converts a slog level into a log level.
func (l *defaultLogger) convLevel(level slog.Level) Level {
	var lv Level
	switch level {
	case slog.LevelWarn:
		lv = LevelWarn
	case slog.LevelDebug:
		lv = LevelDebug
	case slog.LevelInfo:
		lv = LevelInfo
	case slog.LevelError:
		lv = LevelError
	}

	return lv
}
