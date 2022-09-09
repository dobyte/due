/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 11:09 上午
 * @Desc: TODO
 */

package aliyun

import (
	"fmt"
	"strings"
	"time"

	"github.com/aliyun/aliyun-log-go-sdk/producer"

	"github.com/dobyte/due/log"
)

const (
	defaultProject         = "due"
	defaultLogstore        = "app"
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
	opts      *options
	producer  *producer.Producer
	stdLogger log.Logger
}

func NewLogger(opts ...Option) *Logger {
	o := &options{
		project:         defaultProject,
		logstore:        defaultLogstore,
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

	l := &Logger{
		opts:     o,
		producer: producer.InitProducer(config),
		stdLogger: log.NewLogger(
			log.WithOutLevel(o.outLevel),
			log.WithStackLevel(o.stackLevel),
			log.WithCallerFormat(o.callerFormat),
			log.WithTimestampFormat(o.timestampFormat),
			log.WithCallerSkip(o.callerSkip),
		),
	}

	l.producer.Start()

	return l
}

func (l *Logger) log(level log.Level, a ...interface{}) {
	logMap := make(map[string]string)

	var msg string
	if c := len(a); c > 0 {
		msg = fmt.Sprintf(strings.TrimRight(strings.Repeat("%v ", c), " "), a...)
	}

	switch level {
	case log.DebugLevel:
		l.stdLogger.Debug(level, msg)
	}

	now := time.Now()

	logMap[fieldKeyLevel] = lvl.String()[:4]
	logMap[fieldKeyTime] = now.Format(l.opts.timestampFormat)
	logData := producer.GenerateLog(uint32(now.Unix()), logMap)

	_ = l.producer.SendLog(l.opts.project, l.opts.logstore, "", "", logData)
}

// 关闭日志服务
func (l *Logger) Close() {
	l.producer.SafeClose()
}

// Debug 打印调试日志
func (l *Logger) Debug(a ...interface{}) {
	l.log(log.DebugLevel, a...)
}
