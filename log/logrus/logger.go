/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/29 7:24 下午
 * @Desc: TODO
 */

package logrus

import (
	"github.com/dobyte/due/log/logrus/v2/internal/define"
	"github.com/dobyte/due/log/logrus/v2/internal/formatter"
	"github.com/dobyte/due/log/logrus/v2/internal/hook"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/sirupsen/logrus"
	"io"
	"os"
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

	switch o.level {
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

	switch o.format {
	case log.JsonFormat:
		l.logger.SetFormatter(&formatter.JsonFormatter{
			TimeFormat:     o.timeFormat,
			CallerFullPath: o.callerFullPath,
		})
	default:
		l.logger.SetFormatter(&formatter.TextFormatter{
			TimeFormat:     o.timeFormat,
			CallerFullPath: o.callerFullPath,
		})
	}

	l.logger.AddHook(hook.NewStackHook(o.stackLevel, o.callerSkip))

	if o.file != "" {
		if o.classifiedStorage {
			l.logger.AddHook(hook.NewWriterHook(hook.WriterMap{
				logrus.DebugLevel: l.buildWriter(log.DebugLevel),
				logrus.InfoLevel:  l.buildWriter(log.InfoLevel),
				logrus.WarnLevel:  l.buildWriter(log.WarnLevel),
				logrus.ErrorLevel: l.buildWriter(log.ErrorLevel),
				logrus.FatalLevel: l.buildWriter(log.FatalLevel),
				logrus.PanicLevel: l.buildWriter(log.PanicLevel),
			}))
		} else {
			l.logger.AddHook(hook.NewWriterHook(l.buildWriter(log.NoneLevel)))
		}
	}

	if mode.IsDebugMode() && o.stdout {
		l.logger.SetOutput(os.Stdout)
	}

	return l
}

func (l *Logger) buildWriter(level log.Level) io.Writer {
	writer, err := log.NewWriter(log.WriterOptions{
		Path:    l.opts.file,
		Level:   level,
		MaxAge:  l.opts.fileMaxAge,
		MaxSize: l.opts.fileMaxSize * 1024 * 1024,
		CutRule: l.opts.fileCutRule,
	})
	if err != nil {
		panic(err)
	}

	return writer
}

// Print 打印日志，不含堆栈信息
func (l *Logger) Print(level log.Level, a ...interface{}) {
	switch level {
	case log.DebugLevel:
		l.logger.Debug(a...)
	case log.InfoLevel:
		l.logger.Info(a...)
	case log.WarnLevel:
		l.logger.Warn(a...)
	case log.ErrorLevel:
		l.logger.Error(a...)
	case log.FatalLevel:
		l.logger.Fatal(a...)
	case log.PanicLevel:
		l.logger.Panic(a...)
	}
}

// Printf 打印模板日志，不含堆栈信息
func (l *Logger) Printf(level log.Level, format string, a ...interface{}) {
	switch level {
	case log.DebugLevel:
		l.logger.Debugf(format, a...)
	case log.InfoLevel:
		l.logger.Infof(format, a...)
	case log.WarnLevel:
		l.logger.Warnf(format, a...)
	case log.ErrorLevel:
		l.logger.Errorf(format, a...)
	case log.FatalLevel:
		l.logger.Fatalf(format, a...)
	case log.PanicLevel:
		l.logger.Panicf(format, a...)
	}
}

// Debug 打印调试日志
func (l *Logger) Debug(a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Debug(a...)
}

// Debugf 打印调试模板日志
func (l *Logger) Debugf(format string, a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Debugf(format, a...)
}

// Info 打印信息日志
func (l *Logger) Info(a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Info(a...)
}

// Infof 打印信息模板日志
func (l *Logger) Infof(format string, a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Infof(format, a...)
}

// Warn 打印警告日志
func (l *Logger) Warn(a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Warn(a...)
}

// Warnf 打印警告模板日志
func (l *Logger) Warnf(format string, a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Warnf(format, a...)
}

// Error 打印错误日志
func (l *Logger) Error(a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Error(a...)
}

// Errorf 打印错误模板日志
func (l *Logger) Errorf(format string, a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Errorf(format, a...)
}

// Fatal 打印致命错误日志
func (l *Logger) Fatal(a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Fatal(a...)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Fatalf(format, a...)
}

// Panic 打印Panic日志
func (l *Logger) Panic(a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Fatal(a...)
}

// Panicf 打印Panic模板日志
func (l *Logger) Panicf(format string, a ...interface{}) {
	l.logger.WithField(define.StackOutFlagField, true).Fatalf(format, a...)
}
