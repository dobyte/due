package log

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	red    = 31
	yellow = 33
	blue   = 36
	gray   = 37
)

const (
	defaultOutLevel        = WarnLevel
	defaultOutFormat       = TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultFileMaxSize     = 100 * 1024 * 1024
	defaultFileCutRule     = CutByDay
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

type stdLogger struct {
	log  *log.Logger
	opts *options
	pool sync.Pool
}

type entity struct {
	color   int
	level   string
	time    string
	caller  string
	message string
	stack   []string
}

func NewLogger(opts ...Option) Logger {
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

	return &stdLogger{
		opts: o,
		pool: sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
	}
}

func (l *stdLogger) Log(level Level, a ...interface{}) {
	switch l.opts.outFormat {
	case TextFormat:
		l.logText(level, fmt.Sprintf("%v", a))
	case JsonFormat:
		l.logText(level, fmt.Sprintf("%v", a))
	}
}

func (l *stdLogger) logText(level Level, msg string) {
	e := l.buildEntity(level, msg)
	b := l.pool.Get().(*bytes.Buffer)

	fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s]", e.color, e.level, e.time)

	if e.caller != "" {
		fmt.Fprint(b, " "+e.caller)
	}

	if e.message != "" {
		fmt.Fprint(b, " "+e.message)
	}

	fmt.Fprintf(b, "\n")

	NewWriter(WriterOptions{})

	_, _ = os.Stdout.Write(b.Bytes())

	b.Reset()
	l.pool.Put(b)
}

func (l *stdLogger) buildEntity(level Level, msg string) *entity {
	e := &entity{}

	switch level {
	case DebugLevel:
		e.color = gray
	case WarnLevel:
		e.color = yellow
	case ErrorLevel, FatalLevel, PanicLevel:
		e.color = red
	case InfoLevel:
		e.color = blue
	default:
		e.color = blue
	}

	e.level = level.String()[:4]
	e.time = time.Now().Format(l.opts.timestampFormat)
	e.message = strings.TrimRight(msg, "\n")

	if _, file, line, ok := runtime.Caller(2); ok {
		if !l.opts.callerFullPath {
			_, file = filepath.Split(file)
		}
		e.caller = fmt.Sprintf("%s:%d", file, line)
	}

	return e
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
	os.Exit(1)
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
