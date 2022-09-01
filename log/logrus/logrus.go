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
	defaultFileExt         = ".log"
	defaultOutLevel        = log.LevelWarn
	defaultOutFormat       = log.TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultFileMaxSize     = 100 * 1024 * 1024
	defaultFileCutRule     = log.DayCutRule
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

var _ log.Logger = NewLogger()

type logger struct {
	logger *logrus.Logger
}

func NewLogger(opts ...Option) log.Logger {
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

		var filename string
		ext := filepath.Ext(o.outFile)
		if ext == "" {
			filename, ext = srcFilename, defaultFileExt
		} else {
			filename = strings.TrimRight(srcFilename, ext)
		}

		var newFilename string
		var rotationTime time.Duration
		switch o.fileCutRule {
		case log.YearCutRule:
			newFilename = filename + ".%Y" + ext
			rotationTime = 365 * 24 * time.Hour
		case log.MonthCutRule:
			newFilename = filename + ".%Y%m" + ext
			rotationTime = 31 * 24 * time.Hour
		case log.DayCutRule:
			newFilename = filename + ".%Y%m%d" + ext
			rotationTime = 24 * time.Hour
		case log.HourCutRule:
			newFilename = filename + ".%Y%m%d%H" + ext
			rotationTime = time.Hour
		case log.MinuteCutRule:
			newFilename = filename + ".%Y%m%d%H%M" + ext
			rotationTime = time.Minute
		case log.SecondCutRule:
			newFilename = filename + ".%Y%m%d%H%M%S" + ext
			rotationTime = time.Second
		}

		writer, err := rotatelogs.New(
			newFilename,
			rotatelogs.WithLinkName(srcFilename),
			rotatelogs.WithMaxAge(o.fileMaxAge),
			rotatelogs.WithRotationTime(rotationTime),
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
