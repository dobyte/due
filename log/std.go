package log

import (
	"fmt"
	"io"
	"os"
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
	format(e *Entity, isTerminal bool) []byte
}

type Std struct {
	opts       *options
	formatter  formatter
	syncers    []syncer
	bufferPool sync.Pool
	entityPool *EntityPool
}

type enabler func(level Level) bool

type syncer struct {
	writer   io.Writer
	terminal bool
	enabler  enabler
}

var _ Logger = &Std{}

func NewLogger(opts ...Option) *Std {
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

	l := &Std{}
	l.opts = o
	l.syncers = make([]syncer, 0, 7)
	l.entityPool = newEntityPool(l)

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

func (l *Std) buildWriter(level Level) io.Writer {
	w, err := NewWriter(WriterOptions{
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

func (l *Std) buildEnabler(level Level) enabler {
	return func(lvl Level) bool {
		return lvl >= l.opts.outLevel && (level == defaultNoneLevel || (lvl >= level && level >= l.opts.outLevel))
	}
}

// 构建日志实体
func (l *Std) Entity(level Level, a ...interface{}) *Entity {
	return l.entityPool.build(level, a...)
}

// Debug 打印调试日志
func (l *Std) Debug(a ...interface{}) {
	l.Entity(DebugLevel, a...).Log()
}

// Debugf 打印调试模板日志
func (l *Std) Debugf(format string, a ...interface{}) {
	l.Entity(DebugLevel, fmt.Sprintf(format, a...)).Log()
}

// Info 打印信息日志
func (l *Std) Info(a ...interface{}) {
	l.Entity(InfoLevel, a...).Log()
}

// Infof 打印信息模板日志
func (l *Std) Infof(format string, a ...interface{}) {
	l.Entity(InfoLevel, fmt.Sprintf(format, a...)).Log()
}

// Warn 打印警告日志
func (l *Std) Warn(a ...interface{}) {
	l.Entity(WarnLevel, a...).Log()
}

// Warnf 打印警告模板日志
func (l *Std) Warnf(format string, a ...interface{}) {
	l.Entity(WarnLevel, fmt.Sprintf(format, a...)).Log()
}

// Error 打印错误日志
func (l *Std) Error(a ...interface{}) {
	l.Entity(ErrorLevel, a...).Log()
}

// Errorf 打印错误模板日志
func (l *Std) Errorf(format string, a ...interface{}) {
	l.Entity(ErrorLevel, fmt.Sprintf(format, a...)).Log()
}

// Fatal 打印致命错误日志
func (l *Std) Fatal(a ...interface{}) {
	l.Entity(FatalLevel, a...).Log()
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *Std) Fatalf(format string, a ...interface{}) {
	l.Entity(FatalLevel, fmt.Sprintf(format, a...)).Log()
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *Std) Panic(a ...interface{}) {
	l.Entity(PanicLevel, a...).Log()
}

// Panicf 打印Panic模板日志
func (l *Std) Panicf(format string, a ...interface{}) {
	l.Entity(PanicLevel, fmt.Sprintf(format, a...)).Log()
}
