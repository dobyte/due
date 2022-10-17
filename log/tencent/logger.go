/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 6:04 下午
 * @Desc: TODO
 */

package tencent

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	cls "github.com/tencentcloud/tencentcloud-cls-sdk-go"

	"github.com/dobyte/due/log"
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
	logger     interface{}
	producer   *cls.AsyncProducerClient
	bufferPool sync.Pool
}

func NewLogger(opts ...Option) *Logger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	var (
		err      error
		producer *cls.AsyncProducerClient
	)

	if o.syncout {
		config := cls.GetDefaultAsyncProducerClientConfig()
		config.Endpoint = o.endpoint
		config.AccessKeyID = o.accessKeyID
		config.AccessKeySecret = o.accessKeySecret

		producer, err = cls.NewAsyncProducerClient(config)
		if err != nil {
			panic(err)
		}

		producer.Start()
	}

	return &Logger{
		opts:       o,
		producer:   producer,
		bufferPool: sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
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
}

func (l *Logger) log(level log.Level, a ...interface{}) {
	if level < l.opts.level {
		return
	}

	e := l.logger.(interface {
		Entity(log.Level, ...interface{}) *log.Entity
	}).Entity(level, a...)

	if l.opts.syncout {
		logData := cls.NewCLSLog(time.Now().Unix(), l.buildLogRaw(e))
		_ = l.producer.SendLog(l.opts.topicID, logData, nil)
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

// Producer 获取腾讯云Producer客户端
func (l *Logger) Producer() *cls.AsyncProducerClient {
	return l.producer
}

// Close 关闭日志服务
func (l *Logger) Close() error {
	if l.opts.syncout {
		return l.producer.Close(60000)
	}
	return nil
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
	l.Close()
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...interface{}) {
	l.log(log.FatalLevel, fmt.Sprintf(format, a...))
	l.Close()
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
