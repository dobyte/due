package log

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sync"
)

type stdLogger struct {
	log  *log.Logger
	pool sync.Pool
}

func NewLogger(opts ...Option) Logger {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	return &stdLogger{
		log:  log.New(o.writer, o.prefix, o.flag),
		pool: sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
	}
}

func (l *stdLogger) Log(level Level, a ...interface{}) {
	buf := l.pool.Get().(*bytes.Buffer)
	buf.WriteString(fmt.Sprintf("[%s] ", level.String()))
	_, _ = fmt.Fprintf(buf, "%v", a...)
	_ = l.log.Output(3, buf.String())
	buf.Reset()
	l.pool.Put(buf)
}

// Trace 打印事件调试日志
func (l *stdLogger) Trace(a ...interface{}) {
	l.Log(LevelTrace, a...)
}

// Tracef 打印事件调试模板日志
func (l *stdLogger) Tracef(format string, a ...interface{}) {
	l.Log(LevelTrace, fmt.Sprintf(format, a...))
}

// Debug 打印调试日志
func (l *stdLogger) Debug(a ...interface{}) {
	l.Log(LevelDebug, a...)
}

// Debugf 打印调试模板日志
func (l *stdLogger) Debugf(format string, a ...interface{}) {
	l.Log(LevelDebug, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *stdLogger) Info(a ...interface{}) {
	l.Log(LevelInfo, a...)
}

// Infof 打印信息模板日志
func (l *stdLogger) Infof(format string, a ...interface{}) {
	l.Log(LevelInfo, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *stdLogger) Warn(a ...interface{}) {
	l.Log(LevelWarn, a...)
}

// Warnf 打印警告模板日志
func (l *stdLogger) Warnf(format string, a ...interface{}) {
	l.Log(LevelWarn, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *stdLogger) Error(a ...interface{}) {
	l.Log(LevelError, a...)
}

// Errorf 打印错误模板日志
func (l *stdLogger) Errorf(format string, a ...interface{}) {
	l.Log(LevelError, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *stdLogger) Fatal(a ...interface{}) {
	l.Log(LevelFatal, a...)
	os.Exit(0)
}

// Fatalf 打印致命错误模板日志
func (l *stdLogger) Fatalf(format string, a ...interface{}) {
	l.Log(LevelFatal, fmt.Sprintf(format, a...))
	os.Exit(0)
}

// Panic 打印Panic日志
func (l *stdLogger) Panic(a ...interface{}) {
	l.Log(LevelPanic, a...)
	os.Exit(0)
}

// Panicf 打印Panic模板日志
func (l *stdLogger) Panicf(format string, a ...interface{}) {
	l.Log(LevelPanic, fmt.Sprintf(format, a...))
	os.Exit(0)
}
