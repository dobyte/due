/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/29 7:24 下午
 * @Desc: TODO
 */

package logrus

import (
	"github.com/sirupsen/logrus"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/log/utils"
	"io"
	"os"

	"github.com/symsimmy/due/log/logrus/internal/formatter"
	"github.com/symsimmy/due/log/logrus/internal/hook"
	"github.com/symsimmy/due/mode"
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
	case utils.DebugLevel:
		l.logger.SetLevel(logrus.DebugLevel)
	case utils.InfoLevel:
		l.logger.SetLevel(logrus.InfoLevel)
	case utils.WarnLevel:
		l.logger.SetLevel(logrus.WarnLevel)
	case utils.ErrorLevel:
		l.logger.SetLevel(logrus.ErrorLevel)
	case utils.FatalLevel:
		l.logger.SetLevel(logrus.FatalLevel)
	case utils.PanicLevel:
		l.logger.SetLevel(logrus.PanicLevel)
	}

	switch o.format {
	case utils.JsonFormat:
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
				logrus.DebugLevel: l.buildWriter(utils.DebugLevel),
				logrus.InfoLevel:  l.buildWriter(utils.InfoLevel),
				logrus.WarnLevel:  l.buildWriter(utils.WarnLevel),
				logrus.ErrorLevel: l.buildWriter(utils.ErrorLevel),
				logrus.FatalLevel: l.buildWriter(utils.FatalLevel),
				logrus.PanicLevel: l.buildWriter(utils.PanicLevel),
			}))
		} else {
			l.logger.AddHook(hook.NewWriterHook(l.buildWriter(utils.NoneLevel)))
		}
	}

	if mode.IsDebugMode() && o.stdout {
		l.logger.SetOutput(os.Stdout)
	}

	return l
}

func (l *Logger) buildWriter(level utils.Level) io.Writer {
	writer, err := utils.NewWriter(utils.WriterOptions{
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
