package log

import (
	"context"
	"fmt"
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

type terminal struct {
	syncer Syncer
	levels map[Level]bool
}

type defaultLogger struct {
	opts      *options
	pool      *sync.Pool
	terminals []*terminal
}

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
		syncers[syncer.Name()] = syncer
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
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *defaultLogger) Fatalf(format string, a ...any) {
	l.print(LevelFatal, true, fmt.Sprintf(format, a...))
	os.Exit(1)
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

	for i := range l.terminals {
		syncer := l.terminals[i].syncer

		eg.Go(func() error {
			return syncer.Close()
		})
	}

	return eg.Wait()
}

// 打印日志
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

// 释放实体
func (l *defaultLogger) releaseEntity(e *Entity) {
	e.Time = ""
	e.Level = LevelNone
	e.Message = ""
	e.Caller = ""
	e.Frames = nil

	l.pool.Put(e)
}

// 构建实体信息
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

// 构建日志消息
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

// 构建堆栈信息
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
