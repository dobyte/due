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

// Debug 打印调试日志
func (l *stdLogger) Debug(a ...interface{}) {
	l.Log(DebugLevel, a...)
}

// Debugf 打印调试模板日志
func (l *stdLogger) Debugf(format string, a ...interface{}) {
	l.Log(DebugLevel, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *stdLogger) Info(a ...interface{}) {
	l.Log(InfoLevel, a...)
}

// Infof 打印信息模板日志
func (l *stdLogger) Infof(format string, a ...interface{}) {
	l.Log(InfoLevel, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *stdLogger) Warn(a ...interface{}) {
	l.Log(WarnLevel, a...)
}

// Warnf 打印警告模板日志
func (l *stdLogger) Warnf(format string, a ...interface{}) {
	l.Log(WarnLevel, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *stdLogger) Error(a ...interface{}) {
	l.Log(ErrorLevel, a...)
}

// Errorf 打印错误模板日志
func (l *stdLogger) Errorf(format string, a ...interface{}) {
	l.Log(ErrorLevel, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *stdLogger) Fatal(a ...interface{}) {
	l.Log(FatalLevel, a...)
	os.Exit(0)
}

// Fatalf 打印致命错误模板日志
func (l *stdLogger) Fatalf(format string, a ...interface{}) {
	l.Log(FatalLevel, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *stdLogger) Panic(a ...interface{}) {
	l.Log(PanicLevel, a...)
	os.Exit(0)
}

// Panicf 打印Panic模板日志
func (l *stdLogger) Panicf(format string, a ...interface{}) {
	l.Log(PanicLevel, fmt.Sprintf(format, a...))
	os.Exit(0)
}
