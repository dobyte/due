package log

import (
	"fmt"
	"github.com/symsimmy/due/log/utils"
	"io"
	"os"
	"sync"
)

type defaultLogger struct {
	opts       *options
	formatter  formatter
	syncers    []syncer
	bufferPool sync.Pool
	entityPool *EntityPool
}

type enabler func(level utils.Level) bool

type formatter interface {
	format(e *Entity, isTerminal bool) []byte
}

type syncer struct {
	writer   io.Writer
	terminal bool
	enabler  enabler
}

var _ utils.Logger = &defaultLogger{}

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
	case utils.TextFormat:
		l.formatter = newTextFormatter()
	case utils.JsonFormat:
		l.formatter = newJsonFormatter()
	}

	if o.file != "" {
		if o.classifiedStorage {
			l.syncers = append(l.syncers, syncer{
				writer:  l.buildWriter(utils.DebugLevel),
				enabler: l.buildEnabler(utils.DebugLevel),
			}, syncer{
				writer:  l.buildWriter(utils.InfoLevel),
				enabler: l.buildEnabler(utils.InfoLevel),
			}, syncer{
				writer:  l.buildWriter(utils.WarnLevel),
				enabler: l.buildEnabler(utils.WarnLevel),
			}, syncer{
				writer:  l.buildWriter(utils.ErrorLevel),
				enabler: l.buildEnabler(utils.ErrorLevel),
			}, syncer{
				writer:  l.buildWriter(utils.FatalLevel),
				enabler: l.buildEnabler(utils.FatalLevel),
			}, syncer{
				writer:  l.buildWriter(utils.PanicLevel),
				enabler: l.buildEnabler(utils.PanicLevel),
			})
		} else {
			l.syncers = append(l.syncers, syncer{
				writer:  l.buildWriter(utils.NoneLevel),
				enabler: l.buildEnabler(utils.NoneLevel),
			})
		}
	}

	if o.stdout {
		l.syncers = append(l.syncers, syncer{
			writer:   os.Stdout,
			terminal: true,
			enabler:  l.buildEnabler(utils.NoneLevel),
		})
	}

	return l
}

func (l *defaultLogger) buildWriter(level utils.Level) io.Writer {
	w, err := utils.NewWriter(utils.WriterOptions{
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

func (l *defaultLogger) buildEnabler(level utils.Level) enabler {
	return func(lvl utils.Level) bool {
		return lvl >= l.opts.level && (level == utils.NoneLevel || (lvl >= level && level >= l.opts.level))
	}
}

// Entity 构建日志实体
func (l *defaultLogger) Entity(level utils.Level, a ...interface{}) *Entity {
	return l.entityPool.build(level, a...)
}

// Debug 打印调试日志
func (l *defaultLogger) Debug(a ...interface{}) {
	l.Entity(utils.DebugLevel, a...).Log()
}

// Debugf 打印调试模板日志
func (l *defaultLogger) Debugf(format string, a ...interface{}) {
	l.Entity(utils.DebugLevel, fmt.Sprintf(format, a...)).Log()
}

// Info 打印信息日志
func (l *defaultLogger) Info(a ...interface{}) {
	l.Entity(utils.InfoLevel, a...).Log()
}

// Infof 打印信息模板日志
func (l *defaultLogger) Infof(format string, a ...interface{}) {
	l.Entity(utils.InfoLevel, fmt.Sprintf(format, a...)).Log()
}

// Warn 打印警告日志
func (l *defaultLogger) Warn(a ...interface{}) {
	l.Entity(utils.WarnLevel, a...).Log()
}

// Warnf 打印警告模板日志
func (l *defaultLogger) Warnf(format string, a ...interface{}) {
	l.Entity(utils.WarnLevel, fmt.Sprintf(format, a...)).Log()
}

// Error 打印错误日志
func (l *defaultLogger) Error(a ...interface{}) {
	entity := l.Entity(utils.ErrorLevel, a...)
	entity.Log()
}

// Errorf 打印错误模板日志
func (l *defaultLogger) Errorf(format string, a ...interface{}) {
	entity := l.Entity(utils.ErrorLevel, fmt.Sprintf(format, a...))
	entity.Log()
}

// Fatal 打印致命错误日志
func (l *defaultLogger) Fatal(a ...interface{}) {
	entity := l.Entity(utils.FatalLevel, a...)
	entity.Log()
	//os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *defaultLogger) Fatalf(format string, a ...interface{}) {
	entity := l.Entity(utils.FatalLevel, fmt.Sprintf(format, a...))
	entity.Log()
	//os.Exit(1)
}

// Panic 打印Panic日志
func (l *defaultLogger) Panic(a ...interface{}) {
	entity := l.Entity(utils.PanicLevel, a...)
	entity.Log()
}

// Panicf 打印Panic模板日志
func (l *defaultLogger) Panicf(format string, a ...interface{}) {
	entity := l.Entity(utils.PanicLevel, fmt.Sprintf(format, a...))
	entity.Log()
}
