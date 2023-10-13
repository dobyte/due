package log

import (
	"fmt"
	"github.com/dobyte/due/v2/mode"
	"io"
	"os"
	"sync"
)

type Logger interface {
	// Print 打印日志，不含堆栈信息
	Print(level Level, a ...interface{})
	// Printf 打印模板日志，不含堆栈信息
	Printf(level Level, format string, a ...interface{})
	// Debug 打印调试日志
	Debug(a ...interface{})
	// Debugf 打印调试模板日志
	Debugf(format string, a ...interface{})
	// Info 打印信息日志
	Info(a ...interface{})
	// Infof 打印信息模板日志
	Infof(format string, a ...interface{})
	// Warn 打印警告日志
	Warn(a ...interface{})
	// Warnf 打印警告模板日志
	Warnf(format string, a ...interface{})
	// Error 打印错误日志
	Error(a ...interface{})
	// Errorf 打印错误模板日志
	Errorf(format string, a ...interface{})
	// Fatal 打印致命错误日志
	Fatal(a ...interface{})
	// Fatalf 打印致命错误模板日志
	Fatalf(format string, a ...interface{})
	// Panic 打印Panic日志
	Panic(a ...interface{})
	// Panicf 打印Panic模板日志
	Panicf(format string, a ...interface{})
}

type defaultLogger struct {
	opts       *options
	formatter  formatter
	syncers    []syncer
	bufferPool sync.Pool
	entityPool *EntityPool
}

type enabler func(level Level) bool

type formatter interface {
	format(e *Entity, isTerminal bool) []byte
}

type syncer struct {
	writer   io.Writer
	terminal bool
	enabler  enabler
}

var _ Logger = &defaultLogger{}

func NewLogger(opts ...Option) *defaultLogger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	l := &defaultLogger{}
	l.opts = o
	l.syncers = make([]syncer, 0, 7)
	l.entityPool = newEntityPool(l)

	switch l.opts.format {
	case TextFormat:
		l.formatter = newTextFormatter()
	case JsonFormat:
		l.formatter = newJsonFormatter()
	}

	if o.file != "" {
		if o.classifiedStorage {
			l.syncers = append(l.syncers, syncer{
				writer:  l.buildWriter(DebugLevel),
				enabler: l.buildEnabler(DebugLevel),
			}, syncer{
				writer:  l.buildWriter(InfoLevel),
				enabler: l.buildEnabler(InfoLevel),
			}, syncer{
				writer:  l.buildWriter(WarnLevel),
				enabler: l.buildEnabler(WarnLevel),
			}, syncer{
				writer:  l.buildWriter(ErrorLevel),
				enabler: l.buildEnabler(ErrorLevel),
			}, syncer{
				writer:  l.buildWriter(FatalLevel),
				enabler: l.buildEnabler(FatalLevel),
			}, syncer{
				writer:  l.buildWriter(PanicLevel),
				enabler: l.buildEnabler(PanicLevel),
			})
		} else {
			l.syncers = append(l.syncers, syncer{
				writer:  l.buildWriter(NoneLevel),
				enabler: l.buildEnabler(NoneLevel),
			})
		}
	}

	if mode.IsDebugMode() && o.stdout {
		l.syncers = append(l.syncers, syncer{
			writer:   os.Stdout,
			terminal: true,
			enabler:  l.buildEnabler(NoneLevel),
		})
	}

	return l
}

func (l *defaultLogger) buildWriter(level Level) io.Writer {
	w, err := NewWriter(WriterOptions{
		Path:    l.opts.file,
		Level:   level,
		MaxAge:  l.opts.fileMaxAge,
		MaxSize: l.opts.fileMaxSize * 1024 * 1024,
		CutRule: l.opts.fileCutRule,
	})
	if err != nil {
		panic(err)
	}

	return w
}

func (l *defaultLogger) buildEnabler(level Level) enabler {
	return func(lvl Level) bool {
		return lvl >= l.opts.level && (level == NoneLevel || (lvl >= level && level >= l.opts.level))
	}
}

// BuildEntity 构建日志实体
func (l *defaultLogger) BuildEntity(level Level, isNeedStack bool, a ...interface{}) *Entity {
	return l.entityPool.build(level, isNeedStack, a)
}

// 打印日志
func (l *defaultLogger) print(level Level, isNeedStack bool, a ...interface{}) {
	l.BuildEntity(level, isNeedStack, a...).Log()
}

// Print 打印日志
func (l *defaultLogger) Print(level Level, a ...interface{}) {
	l.entityPool.build(level, false, a...).Log()
}

// Printf 打印模板日志
func (l *defaultLogger) Printf(level Level, format string, a ...interface{}) {
	l.print(level, false, fmt.Sprintf(format, a...))
}

// Debug 打印调试日志
func (l *defaultLogger) Debug(a ...interface{}) {
	l.print(DebugLevel, true, a...)
}

// Debugf 打印调试模板日志
func (l *defaultLogger) Debugf(format string, a ...interface{}) {
	l.print(DebugLevel, true, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *defaultLogger) Info(a ...interface{}) {
	l.print(InfoLevel, true, a...)
}

// Infof 打印信息模板日志
func (l *defaultLogger) Infof(format string, a ...interface{}) {
	l.print(InfoLevel, true, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *defaultLogger) Warn(a ...interface{}) {
	l.print(WarnLevel, true, a...)
}

// Warnf 打印警告模板日志
func (l *defaultLogger) Warnf(format string, a ...interface{}) {
	l.print(WarnLevel, true, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *defaultLogger) Error(a ...interface{}) {
	l.print(ErrorLevel, true, a...)
}

// Errorf 打印错误模板日志
func (l *defaultLogger) Errorf(format string, a ...interface{}) {
	l.print(ErrorLevel, true, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *defaultLogger) Fatal(a ...interface{}) {
	l.print(FatalLevel, true, a...)
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *defaultLogger) Fatalf(format string, a ...interface{}) {
	l.print(FatalLevel, true, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *defaultLogger) Panic(a ...interface{}) {
	l.print(PanicLevel, true, a...)
}

// Panicf 打印Panic模板日志
func (l *defaultLogger) Panicf(format string, a ...interface{}) {
	l.print(PanicLevel, true, fmt.Sprintf(format, a...))
}
