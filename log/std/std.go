package std

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/mode"
)

const (
	defaultOutLevel        = InfoLevel
	defaultOutFormat       = TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultFileMaxSize     = 100 * 1024 * 1024
	defaultFileCutRule     = CutByDay
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

const defaultNoneLevel Level = 0

type formatter interface {
	format(e *entity, isTerminal bool) []byte
}

type Logger struct {
	opts       *options
	formatter  formatter
	syncers    []syncer
	bufferPool sync.Pool
	entityPool *Pool
}

type enabler func(level Level) bool

type syncer struct {
	writer   io.Writer
	terminal bool
	enabler  enabler
}

func NewLogger(opts ...Option) *Logger {
	o := &options{
		outLevel:        defaultOutLevel,
		outFormat:       defaultOutFormat,
		fileMaxAge:      defaultFileMaxAge,
		fileMaxSize:     defaultFileMaxSize,
		fileCutRule:     defaultFileCutRule,
		timestampFormat: defaultTimestampFormat,
	}
	for _, opt := range opts {
		opt(o)
	}

	l := &Logger{
		opts:       o,
		entityPool: newEntityPool(o.stackLevel, o.callerFormat, o.timestampFormat, o.callerSkip),
		syncers:    make([]syncer, 0, 7),
	}

	switch l.opts.outFormat {
	case TextFormat:
		l.formatter = newTextFormatter()
	case JsonFormat:
		l.formatter = newJsonFormatter()
	}

	if o.outFile != "" {
		if o.enableLeveledStorage {
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
				writer:  l.buildWriter(defaultNoneLevel),
				enabler: l.buildEnabler(defaultNoneLevel),
			})
		}
	}

	if mode.IsDebugMode() {
		l.syncers = append(l.syncers, syncer{
			writer:   os.Stdout,
			terminal: true,
			enabler:  l.buildEnabler(defaultNoneLevel),
		})
	}

	return l
}

func (l *Logger) log(level Level, a ...interface{}) {
	if level < l.opts.outLevel {
		return
	}

	var msg string
	if c := len(a); c > 0 {
		msg = fmt.Sprintf(strings.TrimRight(strings.Repeat("%v ", c), " "), a...)
	}

	e := l.entityPool.build(level, msg)
	defer e.free()

	buffers := make(map[bool][]byte, 2)
	for _, s := range l.syncers {
		if !s.enabler(level) {
			continue
		}
		b, ok := buffers[s.terminal]
		if !ok {
			b = l.formatter.format(e, s.terminal)
			buffers[s.terminal] = b
		}
		s.writer.Write(b)
	}
}

func (l *Logger) buildWriter(level Level) io.Writer {
	w, err := newWriter(writerOptions{
		Path:    l.opts.outFile,
		Level:   level,
		MaxAge:  l.opts.fileMaxAge,
		MaxSize: l.opts.fileMaxSize,
		CutRule: l.opts.fileCutRule,
	})
	if err != nil {
		panic(err)
	}

	return w
}

func (l *Logger) buildEnabler(level Level) enabler {
	return func(lvl Level) bool {
		return lvl >= l.opts.outLevel && (level == defaultNoneLevel || (lvl >= level && level >= l.opts.outLevel))
	}
}

// Debug 打印调试日志
func (l *Logger) Debug(a ...interface{}) {
	l.log(DebugLevel, a...)
}

// Debugf 打印调试模板日志
func (l *Logger) Debugf(format string, a ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *Logger) Info(a ...interface{}) {
	l.log(InfoLevel, a...)
}

// Infof 打印信息模板日志
func (l *Logger) Infof(format string, a ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *Logger) Warn(a ...interface{}) {
	l.log(WarnLevel, a...)
}

// Warnf 打印警告模板日志
func (l *Logger) Warnf(format string, a ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *Logger) Error(a ...interface{}) {
	l.log(ErrorLevel, a...)
}

// Errorf 打印错误模板日志
func (l *Logger) Errorf(format string, a ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *Logger) Fatal(a ...interface{}) {
	l.log(FatalLevel, a...)
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...interface{}) {
	l.log(FatalLevel, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *Logger) Panic(a ...interface{}) {
	l.log(PanicLevel, a...)
}

// Panicf 打印Panic模板日志
func (l *Logger) Panicf(format string, a ...interface{}) {
	l.log(PanicLevel, fmt.Sprintf(format, a...))
}
