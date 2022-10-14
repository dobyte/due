/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/29 7:24 下午
 * @Desc: TODO
 */

package logrus

import (
	"github.com/sirupsen/logrus"
	"io"
	"os"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/logrus/internal/formatter"
	"github.com/dobyte/due/log/logrus/internal/hook"
	"github.com/dobyte/due/mode"
)

const (
	defaultNoneLevel log.Level = 0
)

var _ log.Logger = NewLogger()

type Logger struct {
	opts   *options
	logger *logrus.Logger
}

func NewLogger(opts ...Option) *Logger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	l := &Logger{opts: o, logger: logrus.New()}

	switch o.outLevel {
	case log.DebugLevel:
		l.logger.SetLevel(logrus.DebugLevel)
	case log.InfoLevel:
		l.logger.SetLevel(logrus.InfoLevel)
	case log.WarnLevel:
		l.logger.SetLevel(logrus.WarnLevel)
	case log.ErrorLevel:
		l.logger.SetLevel(logrus.ErrorLevel)
	case log.FatalLevel:
		l.logger.SetLevel(logrus.FatalLevel)
	case log.PanicLevel:
		l.logger.SetLevel(logrus.PanicLevel)
	}

	switch o.outFormat {
	case log.JsonFormat:
		l.logger.SetFormatter(&formatter.JsonFormatter{
			TimestampFormat: o.timestampFormat,
			CallerFullPath:  o.callerFullPath,
		})
	default:
		l.logger.SetFormatter(&formatter.TextFormatter{
			TimestampFormat: o.timestampFormat,
			CallerFullPath:  o.callerFullPath,
		})
	}

	l.logger.AddHook(hook.NewStackHook(o.outStackLevel))

	if o.outFile != "" {
		if o.fileClassifyStorage {
			l.logger.AddHook(hook.NewWriterHook(hook.WriterMap{
				logrus.DebugLevel: l.buildWriter(log.DebugLevel),
				logrus.InfoLevel:  l.buildWriter(log.InfoLevel),
				logrus.WarnLevel:  l.buildWriter(log.WarnLevel),
				logrus.ErrorLevel: l.buildWriter(log.ErrorLevel),
				logrus.FatalLevel: l.buildWriter(log.FatalLevel),
				logrus.PanicLevel: l.buildWriter(log.PanicLevel),
			}))
		} else {
			l.logger.AddHook(hook.NewWriterHook(l.buildWriter(defaultNoneLevel)))
		}
	}

	if mode.IsDebugMode() {
		l.logger.SetOutput(os.Stdout)
	}

	return l
}

func (l *Logger) buildWriter(level log.Level) io.Writer {
	writer, err := log.NewWriter(log.WriterOptions{
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

// Debug 打印调试日志
func (l *Logger) Debug(a ...interface{}) {
	l.logger.Debug(a...)
}

// Debugf 打印调试模板日志
func (l *Logger) Debugf(format string, a ...interface{}) {
	l.logger.Debugf(format, a...)
}

// Info 打印信息日志
func (l *Logger) Info(a ...interface{}) {
	l.logger.Info(a...)
}

// Infof 打印信息模板日志
func (l *Logger) Infof(format string, a ...interface{}) {
	l.logger.Infof(format, a...)
}

// Warn 打印警告日志
func (l *Logger) Warn(a ...interface{}) {
	l.logger.Warn(a...)
}

// Warnf 打印警告模板日志
func (l *Logger) Warnf(format string, a ...interface{}) {
	l.logger.Warnf(format, a...)
}

// Error 打印错误日志
func (l *Logger) Error(a ...interface{}) {
	l.logger.Error(a...)
}

// Errorf 打印错误模板日志
func (l *Logger) Errorf(format string, a ...interface{}) {
	l.logger.Errorf(format, a...)
}

// Fatal 打印致命错误日志
func (l *Logger) Fatal(a ...interface{}) {
	l.logger.Fatal(a...)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...interface{}) {
	l.logger.Fatalf(format, a...)
}

// Panic 打印Panic日志
func (l *Logger) Panic(a ...interface{}) {
	l.logger.Fatal(a...)
}

// Panicf 打印Panic模板日志
func (l *Logger) Panicf(format string, a ...interface{}) {
	l.logger.Fatalf(format, a...)
}
