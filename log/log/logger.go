package log

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/stack"
	"golang.org/x/sync/errgroup"
)

type Logger interface {
	// Print 打印日志，不含堆栈信息
	Print(level Level, a ...any)
	// Printf 打印模板日志，不含堆栈信息
	Printf(level Level, format string, a ...any)
	// Debug 打印调试日志
	Debug(a ...any)
	// Debugf 打印调试模板日志
	Debugf(format string, a ...any)
	// Info 打印信息日志
	Info(a ...any)
	// Infof 打印信息模板日志
	Infof(format string, a ...any)
	// Warn 打印警告日志
	Warn(a ...any)
	// Warnf 打印警告模板日志
	Warnf(format string, a ...any)
	// Error 打印错误日志
	Error(a ...any)
	// Errorf 打印错误模板日志
	Errorf(format string, a ...any)
	// Fatal 打印致命错误日志
	Fatal(a ...any)
	// Fatalf 打印致命错误模板日志
	Fatalf(format string, a ...any)
	// Panic 打印Panic日志
	Panic(a ...any)
	// Panicf 打印Panic模板日志
	Panicf(format string, a ...any)
	// Close 关闭日志
	Close() error
}

type defaultLogger struct {
	opts      *options
	pool      *sync.Pool
	loc       *time.Location
	syncers   []Syncer
	formatter Formatter
}

func NewLogger(opts ...Option) *defaultLogger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	l := &defaultLogger{}
	l.opts = o
	l.pool = &sync.Pool{New: func() any { return &Entity{} }}

	if formatter, ok := formatters[string(o.outFormat)]; ok {
		l.formatter = formatter
	} else {
		l.formatter = formatters[string(FormatText)]
	}

	for _, terminal := range l.opts.outTerminals {
		if syncer, ok := syncers[string(terminal)]; ok {
			l.syncers = append(l.syncers, syncer)
		}
	}

	if loc, err := time.LoadLocation(l.opts.timeZone); err != nil {
		l.loc = time.Local
	} else {
		l.loc = loc
	}

	return l
}

// Print 打印日志，不含堆栈信息
func (l *defaultLogger) Print(level Level, a ...any) {
	l.print(level, false, a...)
}

// Printf 打印模板日志，不含堆栈信息
func (l *defaultLogger) Printf(level Level, format string, a ...any) {
	l.print(level, false, fmt.Sprintf(format, a...))
}

// Debug 打印调试日志
func (l *defaultLogger) Debug(a ...any) {
	l.print(LevelDebug, true, a...)
}

// Debugf 打印调试模板日志
func (l *defaultLogger) Debugf(format string, a ...any) {
	l.print(LevelDebug, true, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *defaultLogger) Info(a ...any) {
	l.print(LevelInfo, true, a...)
}

// Infof 打印信息模板日志
func (l *defaultLogger) Infof(format string, a ...any) {
	l.print(LevelInfo, true, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *defaultLogger) Warn(a ...any) {
	l.print(LevelWarn, true, a...)
}

// Warnf 打印警告模板日志
func (l *defaultLogger) Warnf(format string, a ...any) {
	l.print(LevelWarn, true, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *defaultLogger) Error(a ...any) {
	l.print(LevelError, true, a...)
}

// Errorf 打印错误模板日志
func (l *defaultLogger) Errorf(format string, a ...any) {
	l.print(LevelError, true, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *defaultLogger) Fatal(a ...any) {
	l.print(LevelFatal, true, a...)
}

// Fatalf 打印致命错误模板日志
func (l *defaultLogger) Fatalf(format string, a ...any) {
	l.print(LevelFatal, true, fmt.Sprintf(format, a...))
}

// Panic 打印Panic日志
func (l *defaultLogger) Panic(a ...any) {
	l.print(LevelPanic, true, a...)
}

// Panicf 打印Panic模板日志
func (l *defaultLogger) Panicf(format string, a ...any) {
	l.print(LevelPanic, true, fmt.Sprintf(format, a...))
}

// Close 关闭日志
func (l *defaultLogger) Close() error {
	eg, _ := errgroup.WithContext(context.Background())

	for i := range l.syncers {
		syncer := l.syncers[i]

		eg.Go(func() error {
			return syncer.Close()
		})
	}

	return eg.Wait()
}

// 打印日志
func (l *defaultLogger) print(level Level, isOutStack bool, a ...any) {
	n := len(l.syncers)

	if n == 0 {
		return
	}

	if level.Priority() < l.opts.outLevel.Priority() {
		return
	}

	entity := l.buildEntity(level, isOutStack, a...)
	defer entity.Release()

	if n == 1 {
		syncer := l.syncers[0]

		buf := l.formatter.Format(entity, syncer.Name() == string(TerminalConsole))
		defer buf.Release()

		syncer.Write(buf.Bytes())
	} else {
		var (
			buf1  *Buffer
			buf2  *Buffer
			eg, _ = errgroup.WithContext(context.Background())
		)

		for i := range l.syncers {
			syncer := l.syncers[i]

			if syncer.Name() == string(TerminalConsole) {
				buf1 = l.formatter.Format(entity, true)
			} else {
				buf2 = l.formatter.Format(entity)
			}

			eg.Go(func() error {
				var err error

				if syncer.Name() == string(TerminalConsole) {
					_, err = syncer.Write(buf1.Bytes())
				} else {
					_, err = syncer.Write(buf2.Bytes())
				}

				return err
			})
		}

		_ = eg.Wait()

		if buf1 != nil {
			buf1.Release()
		}

		if buf2 != nil {
			buf2.Release()
		}
	}
}

// 构建实体信息
func (l *defaultLogger) buildEntity(level Level, isOutStack bool, a ...any) *Entity {
	e := l.pool.Get().(*Entity)
	e.pool = l.pool
	e.level = level
	e.time = l.buildTime()
	e.message = l.buildMessage(a...)

	if isOutStack && l.opts.outStackLevel != "" && l.opts.outStackLevel != LevelNone && level.Priority() >= l.opts.outStackLevel.Priority() {
		e.caller, e.frames = l.buildStack(stack.Full)
	} else {
		e.caller, e.frames = l.buildStack(stack.First)
	}

	return e
}

// 构建时间
func (l *defaultLogger) buildTime() string {
	return l.now().Format(l.opts.timeFormat)
}

// 构建日志消息
func (l *defaultLogger) buildMessage(a ...any) string {
	if c := len(a); c > 0 {
		return strings.TrimSuffix(fmt.Sprintf(strings.TrimSuffix(strings.Repeat("%v ", c), " "), a...), "\n")
	} else {
		return ""
	}
}

// 构建堆栈信息
func (l *defaultLogger) buildStack(depth stack.Depth) (string, []runtime.Frame) {
	st := stack.Callers(3+l.opts.outCallerDepth, depth)
	defer st.Free()

	var (
		caller string
		frames = st.Frames()
	)

	if len(frames) > 0 {
		file := frames[0].File
		line := frames[0].Line

		if !l.opts.outCallerFullPath {
			_, file = filepath.Split(file)
		}

		caller = fmt.Sprintf("%s:%d", file, line)
	}

	if depth == stack.First {
		return caller, nil
	} else {
		return caller, frames
	}
}

// 获取当前时间
func (l *defaultLogger) now() time.Time {
	return time.Now().In(l.loc)
}
