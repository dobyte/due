/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/29 7:24 下午
 * @Desc: TODO
 */

package logrus

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"

	"github.com/lestrrat-go/file-rotatelogs"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/logrus/internal/formatter"
)

const (
	defaultOutSize         = 100 * 1024 * 1024
	defaultOutLevel        = log.LevelWarn
	defaultOutFormat       = log.TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

var _ log.Logger = NewLogger()

type logger struct {
	logger *logrus.Logger
}

func NewLogger(opts ...Option) log.Logger {
	o := &options{
		outSize:         defaultOutSize,
		outLevel:        defaultOutLevel,
		outFormat:       defaultOutFormat,
		fileMaxAge:      defaultFileMaxAge,
		timestampFormat: defaultTimestampFormat,
	}
	for _, opt := range opts {
		opt(o)
	}

	l := &logger{logger: logrus.New()}

	switch o.outLevel {
	case log.LevelTrace:
		l.logger.SetLevel(logrus.TraceLevel)
	case log.LevelDebug:
		l.logger.SetLevel(logrus.DebugLevel)
	case log.LevelInfo:
		l.logger.SetLevel(logrus.InfoLevel)
	case log.LevelWarn:
		l.logger.SetLevel(logrus.WarnLevel)
	case log.LevelError:
		l.logger.SetLevel(logrus.ErrorLevel)
	case log.LevelFatal:
		l.logger.SetLevel(logrus.FatalLevel)
	case log.LevelPanic:
		l.logger.SetLevel(logrus.PanicLevel)
	}

	var f logrus.Formatter
	switch o.outFormat {
	case log.JsonFormat:
		f = &formatter.JsonFormatter{
			TimestampFormat: o.timestampFormat,
			CallerFullPath:  o.callerFullPath,
		}
	default:
		f = &formatter.TextFormatter{
			TimestampFormat: o.timestampFormat,
			CallerFullPath:  o.callerFullPath,
		}
	}

	if o.outFile != "" {
		_, err := os.OpenFile(o.outFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			log.Fatal(err)
		}

		_, srcFilename := filepath.Split(o.outFile)

		var newFilename string
		if ext := filepath.Ext(o.outFile); ext == "" {
			newFilename = srcFilename + ".%Y%m%d.log"
		} else {
			newFilename = strings.TrimRight(srcFilename, ext) + ".%Y%m%d" + ext
		}

		writer, err := rotatelogs.New(
			newFilename,
			rotatelogs.WithLinkName(srcFilename),
			rotatelogs.WithMaxAge(o.fileMaxAge),
			rotatelogs.WithRotationTime(24*time.Hour),
			rotatelogs.WithRotationSize(o.fileMaxSize),
		)
		if err != nil {
			log.Fatal(err)
		}

		l.logger.AddHook(lfshook.NewHook(lfshook.WriterMap{
			logrus.TraceLevel: writer,
			logrus.DebugLevel: writer,
			logrus.InfoLevel:  writer,
			logrus.WarnLevel:  writer,
			logrus.ErrorLevel: writer,
			logrus.FatalLevel: writer,
			logrus.PanicLevel: writer,
		}, f))
	}

	l.logger.SetFormatter(f)
	l.logger.SetOutput(os.Stdout)
	l.logger.SetReportCaller(true)

	return l
}

// Trace 打印事件调试日志
func (l *logger) Trace(a ...interface{}) {
	l.logger.Trace(a...)
}

// Tracef 打印事件调试模板日志
func (l *logger) Tracef(format string, a ...interface{}) {
	l.logger.Tracef(format, a...)
}

// Debug 打印调试日志
func (l *logger) Debug(a ...interface{}) {
	l.logger.Debug(a...)
}

// Debugf 打印调试模板日志
func (l *logger) Debugf(format string, a ...interface{}) {
	l.logger.Debugf(format, a...)
}

// Info 打印信息日志
func (l *logger) Info(a ...interface{}) {
	l.logger.Info(a...)
}

// Infof 打印信息模板日志
func (l *logger) Infof(format string, a ...interface{}) {
	l.logger.Infof(format, a...)
}

// Warn 打印警告日志
func (l *logger) Warn(a ...interface{}) {
	l.logger.Warn(a...)
}

// Warnf 打印警告模板日志
func (l *logger) Warnf(format string, a ...interface{}) {
	l.logger.Warnf(format, a...)
}

// Error 打印错误日志
func (l *logger) Error(a ...interface{}) {
	l.logger.Error(a...)
}

// Errorf 打印错误模板日志
func (l *logger) Errorf(format string, a ...interface{}) {
	l.logger.Errorf(format, a...)
}

// Fatal 打印致命错误日志
func (l *logger) Fatal(a ...interface{}) {
	l.logger.Fatal(a...)
}

// Fatalf 打印致命错误模板日志
func (l *logger) Fatalf(format string, a ...interface{}) {
	l.logger.Fatalf(format, a...)
}

// Panic 打印Panic日志
func (l *logger) Panic(a ...interface{}) {
	l.logger.Fatal(a...)
}

// Panicf 打印Panic模板日志
func (l *logger) Panicf(format string, a ...interface{}) {
	l.logger.Fatalf(format, a...)
}
