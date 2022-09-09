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
	"time"

	"github.com/aliyun/aliyun-log-go-sdk/producer"

	"github.com/dobyte/due/log"
)

const (
	defaultOutLevel        = log.InfoLevel
	defaultCallerFormat    = log.CallerShortPath
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
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
	std        *log.Std
	producer   *producer.Producer
	bufferPool sync.Pool
}

func NewLogger(opts ...Option) *Logger {
	o := &options{
		outLevel:        defaultOutLevel,
		callerFormat:    defaultCallerFormat,
		timestampFormat: defaultTimestampFormat,
	}
	for _, opt := range opts {
		opt(o)
	}

	config := producer.GetDefaultProducerConfig()
	config.Endpoint = o.endpoint
	config.AccessKeyID = o.accessKeyID
	config.AccessKeySecret = o.accessKeySecret
	config.AllowLogLevel = "error"

	l := &Logger{
		opts:       o,
		producer:   producer.InitProducer(config),
		bufferPool: sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
		std: log.NewLogger(
			log.WithOutLevel(o.outLevel),
			log.WithStackLevel(o.stackLevel),
			log.WithCallerFormat(o.callerFormat),
			log.WithTimestampFormat(o.timestampFormat),
			log.WithCallerSkip(o.callerSkip+1),
		),
	}

	l.producer.Start()

	return l
}

func (l *Logger) log(level log.Level, a ...interface{}) {
	if level < l.opts.outLevel {
		return
	}

	e := l.std.Entity(level, a...)

	if !l.opts.disableSyncing {
		logData := producer.GenerateLog(uint32(time.Now().Unix()), l.buildLogRaw(e))
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

// 关闭日志服务
func (l *Logger) Close() error {
	return l.producer.Close(5000)
}

// Debug 打印调试日志
func (l *Logger) Debug(a ...interface{}) {
	l.log(log.DebugLevel, a...)
}

// Debugf 打印调试模板日志
func (l *Logger) Debugf(format string, a ...interface{}) {
	l.log(log.DebugLevel, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *Logger) Info(a ...interface{}) {
	l.log(log.InfoLevel, a...)
}

// Infof 打印信息模板日志
func (l *Logger) Infof(format string, a ...interface{}) {
	l.log(log.InfoLevel, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *Logger) Warn(a ...interface{}) {
	l.log(log.WarnLevel, a...)
}

// Warnf 打印警告模板日志
func (l *Logger) Warnf(format string, a ...interface{}) {
	l.log(log.WarnLevel, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *Logger) Error(a ...interface{}) {
	l.log(log.ErrorLevel, a...)
}

// Errorf 打印错误模板日志
func (l *Logger) Errorf(format string, a ...interface{}) {
	l.log(log.ErrorLevel, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *Logger) Fatal(a ...interface{}) {
	l.log(log.FatalLevel, a...)
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...interface{}) {
	l.log(log.FatalLevel, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// Panic 打印Panic日志
func (l *Logger) Panic(a ...interface{}) {
	l.log(log.PanicLevel, a...)
}

// Panicf 打印Panic模板日志
func (l *Logger) Panicf(format string, a ...interface{}) {
	l.log(log.PanicLevel, fmt.Sprintf(format, a...))
}
