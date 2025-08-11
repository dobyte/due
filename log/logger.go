package log

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dobyte/due/v2/mode"
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
func (l *defaultLogger) BuildEntity(level Level, isNeedStack bool, a ...any) *Entity {
	return l.entityPool.build(level, isNeedStack, a...)
}

// 打印日志
func (l *defaultLogger) print(level Level, isNeedStack bool, a ...any) {
	l.BuildEntity(level, isNeedStack, a...).Log()
}

// Print 打印日志
func (l *defaultLogger) Print(level Level, a ...any) {
	l.print(level, false, a...)
}

// Printf 打印模板日志
func (l *defaultLogger) Printf(level Level, format string, a ...any) {
	l.print(level, false, fmt.Sprintf(format, a...))
}

// Debug 打印调试日志
func (l *defaultLogger) Debug(a ...any) {
	l.print(DebugLevel, true, a...)
}

// Debugf 打印调试模板日志
func (l *defaultLogger) Debugf(format string, a ...any) {
	l.print(DebugLevel, true, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *defaultLogger) Info(a ...any) {
	l.print(InfoLevel, true, a...)
}

// Infof 打印信息模板日志
func (l *defaultLogger) Infof(format string, a ...any) {
	l.print(InfoLevel, true, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *defaultLogger) Warn(a ...any) {
	l.print(WarnLevel, true, a...)
}

// Warnf 打印警告模板日志
func (l *defaultLogger) Warnf(format string, a ...any) {
	l.print(WarnLevel, true, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *defaultLogger) Error(a ...any) {
	l.print(ErrorLevel, true, a...)
}

// Errorf 打印错误模板日志
func (l *defaultLogger) Errorf(format string, a ...any) {
	l.print(ErrorLevel, true, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *defaultLogger) Fatal(a ...any) {
	l.print(FatalLevel, true, a...)
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *defaultLogger) Fatalf(format string, a ...any) {
	l.print(FatalLevel, true, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *defaultLogger) Panic(a ...any) {
	l.print(PanicLevel, true, a...)
}

// Panicf 打印Panic模板日志
func (l *defaultLogger) Panicf(format string, a ...any) {
	l.print(PanicLevel, true, fmt.Sprintf(format, a...))
}

// Close 关闭日志
func (l *defaultLogger) Close() (err error) {
	for _, s := range l.syncers {
		w, ok := s.writer.(interface{ Close() error })
		if !ok {
			continue
		}

		if e := w.Close(); e != nil {
			err = e
		}
	}

	return
}
