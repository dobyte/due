package log

import (
	"fmt"
	"github.com/symsimmy/due/mode"
	"io"
	"os"
	"sync"
)

type Logger interface {
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

// Entity 构建日志实体
func (l *defaultLogger) Entity(level Level, a ...interface{}) *Entity {
	return l.entityPool.build(level, a...)
}

// Debug 打印调试日志
func (l *defaultLogger) Debug(a ...interface{}) {
	l.Entity(DebugLevel, a...).Log()
}

// Debugf 打印调试模板日志
func (l *defaultLogger) Debugf(format string, a ...interface{}) {
	l.Entity(DebugLevel, fmt.Sprintf(format, a...)).Log()
}

// Info 打印信息日志
func (l *defaultLogger) Info(a ...interface{}) {
	l.Entity(InfoLevel, a...).Log()
}

// Infof 打印信息模板日志
func (l *defaultLogger) Infof(format string, a ...interface{}) {
	l.Entity(InfoLevel, fmt.Sprintf(format, a...)).Log()
}

// Warn 打印警告日志
func (l *defaultLogger) Warn(a ...interface{}) {
	l.Entity(WarnLevel, a...).Log()
}

// Warnf 打印警告模板日志
func (l *defaultLogger) Warnf(format string, a ...interface{}) {
	l.Entity(WarnLevel, fmt.Sprintf(format, a...)).Log()
}

// Error 打印错误日志
func (l *defaultLogger) Error(a ...interface{}) {
	l.Entity(ErrorLevel, a...).Log()
}

// Errorf 打印错误模板日志
func (l *defaultLogger) Errorf(format string, a ...interface{}) {
	l.Entity(ErrorLevel, fmt.Sprintf(format, a...)).Log()
}

// Fatal 打印致命错误日志
func (l *defaultLogger) Fatal(a ...interface{}) {
	l.Entity(FatalLevel, a...).Log()
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *defaultLogger) Fatalf(format string, a ...interface{}) {
	l.Entity(FatalLevel, fmt.Sprintf(format, a...)).Log()
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *defaultLogger) Panic(a ...interface{}) {
	l.Entity(PanicLevel, a...).Log()
}

// Panicf 打印Panic模板日志
func (l *defaultLogger) Panicf(format string, a ...interface{}) {
	l.Entity(PanicLevel, fmt.Sprintf(format, a...)).Log()
}
