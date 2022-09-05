package log

import (
	"bytes"
	"fmt"
	"io"
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

const defaultNoneLevel Level = 0

type stdLogger struct {
	log     *log.Logger
	opts    *options
	pool    sync.Pool
	syncers []syncer
}

type enabler func(level Level) bool

type syncer struct {
	writer   io.Writer
	terminal bool
	enabler  enabler
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

	l := &stdLogger{
		opts:    o,
		pool:    sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
		syncers: make([]syncer, 0, 7),
	}

	if o.outFile != "" {
		if o.fileClassifyStorage {
			l.syncers = append(l.syncers, syncer{
				writer:   os.Stdout,
				terminal: true,
				enabler:  l.buildEnabler(defaultNoneLevel),
			}, syncer{
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
				writer:   os.Stdout,
				terminal: true,
				enabler:  l.buildEnabler(defaultNoneLevel),
			}, syncer{
				writer:  l.buildWriter(defaultNoneLevel),
				enabler: l.buildEnabler(defaultNoneLevel),
			})
		}
	} else {
		l.syncers = append(l.syncers, syncer{
			writer:   os.Stdout,
			terminal: true,
			enabler:  l.buildEnabler(defaultNoneLevel),
		})
	}

	return l
}

func (l *stdLogger) Log(level Level, a ...interface{}) {
	if level < l.opts.outLevel {
		return
	}

	switch l.opts.outFormat {
	case TextFormat:
		l.logText(level, fmt.Sprintf("%v", a))
	case JsonFormat:
		l.logJson(level, fmt.Sprintf("%v", a))
	}
}

func (l *stdLogger) logText(level Level, msg string) {
	e := l.buildEntity(level, msg)
	buffers := make(map[bool]*bytes.Buffer, 2)

	for _, s := range l.syncers {
		if !s.enabler(level) {
			continue
		}
		b, ok := buffers[s.terminal]
		if !ok {
			b = l.buildTextBuffer(e, s.terminal)
			buffers[s.terminal] = b
		}
		s.writer.Write(b.Bytes())
	}

	for _, b := range buffers {
		b.Reset()
		l.pool.Put(b)
	}
}

func (l *stdLogger) logJson(level Level, msg string) {
	e := l.buildEntity(level, msg)
	b := l.buildJsonBuffer(e)

	for _, s := range l.syncers {
		if !s.enabler(level) {
			continue
		}
		s.writer.Write(b.Bytes())
	}

	b.Reset()
	l.pool.Put(b)
}

func (l *stdLogger) buildTextBuffer(e *entity, isTerminal bool) *bytes.Buffer {
	b := l.pool.Get().(*bytes.Buffer)

	if isTerminal {
		_, _ = fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s]", e.color, e.level, e.time)
	} else {
		_, _ = fmt.Fprintf(b, "%s[%s]", e.level, e.time)
	}

	if e.caller != "" {
		_, _ = fmt.Fprint(b, " "+e.caller)
	}

	if e.message != "" {
		_, _ = fmt.Fprint(b, " "+e.message)
	}

	_, _ = fmt.Fprintf(b, "\n")

	return b
}

func (l *stdLogger) buildJsonBuffer(e *entity) *bytes.Buffer {
	b := l.pool.Get().(*bytes.Buffer)

	return b
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

	pcs := make([]uintptr, 5)
	num := runtime.Callers(4, pcs)

	for _, pc := range pcs[:num] {
		fun := runtime.FuncForPC(pc)
		file, line := fun.FileLine(pc - 1)
		fmt.Println(fun.Name(), file, line)
	}

	//debug.PrintStack()

	return e
}

func (l *stdLogger) buildWriter(level Level) io.Writer {
	writer, err := NewWriter(WriterOptions{
		Path:    l.opts.outFile,
		Level:   level,
		MaxAge:  l.opts.fileMaxAge,
		MaxSize: l.opts.fileMaxSize,
		CutRule: l.opts.fileCutRule,
	})
	if err != nil {
		panic(err)
	}

	return writer
}

func (l *stdLogger) buildEnabler(level Level) enabler {
	return func(lvl Level) bool {
		return lvl >= l.opts.outLevel && (level == defaultNoneLevel || (lvl >= level && level >= l.opts.outLevel))
	}
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
