/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 11:09 上午
 * @Desc: TODO
 */

package aliyun

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/aliyun/aliyun-log-go-sdk/producer"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
)

const (
	fieldKeyLevel     = "level"
	fieldKeyTime      = "time"
	fieldKeyFile      = "file"
	fieldKeyMsg       = "msg"
	fieldKeyStack     = "stack"
	fieldKeyStackFunc = "func"
	fieldKeyStackFile = "file"
)

type Logger struct {
	opts       *options
	logger     any
	producer   *producer.Producer
	bufferPool sync.Pool
}

func NewLogger(opts ...Option) *Logger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	l := &Logger{
		opts:       o,
		bufferPool: sync.Pool{New: func() any { return &bytes.Buffer{} }},
		logger: log.NewLogger(
			log.WithFile(""),
			log.WithLevel(o.level),
			log.WithFormat(log.TextFormat),
			log.WithStdout(o.stdout),
			log.WithTimeFormat(o.timeFormat),
			log.WithStackLevel(o.stackLevel),
			log.WithCallerFullPath(o.callerFullPath),
			log.WithCallerSkip(o.callerSkip+1),
		),
	}

	if o.syncout {
		config := producer.GetDefaultProducerConfig()
		config.Endpoint = o.endpoint
		config.AccessKeyID = o.accessKeyID
		config.AccessKeySecret = o.accessKeySecret
		config.AllowLogLevel = "error"

		l.producer = producer.InitProducer(config)
		l.producer.Start()
	}

	return l
}

func (l *Logger) print(level log.Level, isNeedStack bool, a ...any) {
	if level < l.opts.level {
		return
	}

	e := l.logger.(interface {
		BuildEntity(log.Level, bool, ...any) *log.Entity
	}).BuildEntity(level, isNeedStack, a...)

	if l.opts.syncout {
		logData := producer.GenerateLog(uint32(xtime.Now().Unix()), l.buildLogRaw(e))
		_ = l.producer.SendLog(l.opts.project, l.opts.logstore, l.opts.topic, l.opts.source, logData)
	}

	e.Log()
}

func (l *Logger) buildLogRaw(e *log.Entity) map[string]string {
	raw := make(map[string]string)
	raw[fieldKeyLevel] = e.Level.String()[:4]
	raw[fieldKeyTime] = e.Time
	raw[fieldKeyFile] = e.Caller
	raw[fieldKeyMsg] = e.Message

	if len(e.Frames) > 0 {
		b := l.bufferPool.Get().(*bytes.Buffer)
		defer func() {
			b.Reset()
			l.bufferPool.Put(b)
		}()

		fmt.Fprint(b, "[")
		for i, frame := range e.Frames {
			if i == 0 {
				fmt.Fprintf(b, `{"%s":"%s"`, fieldKeyStackFunc, frame.Function)
			} else {
				fmt.Fprintf(b, `,{"%s":"%s"`, fieldKeyStackFunc, frame.Function)
			}
			fmt.Fprintf(b, `,"%s":"%s:%d"}`, fieldKeyStackFile, frame.File, frame.Line)
		}
		fmt.Fprint(b, "]")

		raw[fieldKeyStack] = b.String()
	}

	return raw
}

// Producer 获取阿里云Producer
func (l *Logger) Producer() *producer.Producer {
	return l.producer
}

// Close 关闭日志服务
func (l *Logger) Close() error {
	if l.opts.syncout {
		return l.producer.Close(5000)
	}
	return nil
}

// Print 打印日志，不含堆栈信息
func (l *Logger) Print(level log.Level, a ...any) {
	l.print(level, false, a...)
}

// Printf 打印模板日志，不含堆栈信息
func (l *Logger) Printf(level log.Level, format string, a ...any) {
	l.print(level, false, fmt.Sprintf(format, a...))
}

// Debug 打印调试日志
func (l *Logger) Debug(a ...any) {
	l.print(log.DebugLevel, true, a...)
}

// Debugf 打印调试模板日志
func (l *Logger) Debugf(format string, a ...any) {
	l.print(log.DebugLevel, true, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *Logger) Info(a ...any) {
	l.print(log.InfoLevel, true, a...)
}

// Infof 打印信息模板日志
func (l *Logger) Infof(format string, a ...any) {
	l.print(log.InfoLevel, true, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *Logger) Warn(a ...any) {
	l.print(log.WarnLevel, true, a...)
}

// Warnf 打印警告模板日志
func (l *Logger) Warnf(format string, a ...any) {
	l.print(log.WarnLevel, true, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *Logger) Error(a ...any) {
	l.print(log.ErrorLevel, true, a...)
}

// Errorf 打印错误模板日志
func (l *Logger) Errorf(format string, a ...any) {
	l.print(log.ErrorLevel, true, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *Logger) Fatal(a ...any) {
	l.print(log.FatalLevel, true, a...)
	l.Close()
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...any) {
	l.print(log.FatalLevel, true, fmt.Sprintf(format, a...))
	l.Close()
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *Logger) Panic(a ...any) {
	l.print(log.PanicLevel, true, a...)
}

// Panicf 打印Panic模板日志
func (l *Logger) Panicf(format string, a ...any) {
	l.print(log.PanicLevel, true, fmt.Sprintf(format, a...))
}
