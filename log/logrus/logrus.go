/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/29 7:24 下午
 * @Desc: TODO
 */

package logrus

import (
	"github.com/sirupsen/logrus"
	
	"github.com/dobyte/due/log"
)

type logger struct {
	logger *logrus.Logger
}

func NewLogger(opts ...Option) log.Logger {
	o := &options{
		level: log.LevelWarn,
	}
	for _, opt := range opts {
		opt(o)
	}
	
	l := &logger{logger: logrus.New()}
	
	switch o.level {
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
	
	return l
}

// Info 打印信息日志
func (l *logger) Info(a ...interface{}) {
	l.logger.Info(a...)
}

// Infof 打印信息模板日志
func (l *logger) Infof(format string, a ...interface{}) {
	l.logger.Infof(format, a...)
}

// Debug 打印调试日志
func (l *logger) Debug(a ...interface{}) {
	l.logger.Debug(a...)
}

// Debugf 打印调试模板日志
func (l *logger) Debugf(format string, a ...interface{}) {
	l.logger.Debugf(format, a...)
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
