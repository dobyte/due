package log

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/log"
	"github.com/dobyte/due/v2/core/stack"
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

type Formatter interface {
	Format(e *Entity, isTerminal bool) []byte
}

type defaultLogger struct {
	opts *options
	pool *sync.Pool
	loc  *time.Location

	writer *log.Writer
}

func NewLogger(opts ...Option) *defaultLogger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	l := &defaultLogger{}
	l.opts = o
	l.pool = &sync.Pool{New: func() any { return &entity{} }}

	return l
}

// 构建实体信息
func (l *defaultLogger) buildEntity(level Level, isOutStack bool, a ...any) *entity {
	e := l.pool.Get().(*entity)
	e.pool = l.pool
	e.level = level
	e.time = l.now()
	e.message = l.buildMessage(a...)

	if isOutStack && l.opts.outStackLevel != "" && l.opts.outStackLevel != LevelNone && level.Priority() >= l.opts.outStackLevel.Priority() {
		e.caller, e.frames = l.buildStack(stack.Full)
	} else {
		e.caller, e.frames = l.buildStack(stack.First)
	}

	return e
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
