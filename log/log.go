package log

import (
	"fmt"
	"github.com/symsimmy/due/env"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/log/utils"
	"github.com/symsimmy/due/log/zap"
	"os"
	"strings"
)

const (
	enableAsyncLogKey = "ASYNC_LOG"
)

var globalLogger utils.Logger

func init() {
	enableAsyncLog := env.Get(enableAsyncLogKey, true).Bool()
	if enableAsyncLog {
		logger := zap.NewAsyncLogger(zap.WithCallerSkip(1))
		SetLogger(logger)
	} else {
		logger := zap.NewLogger(zap.WithCallerSkip(1))
		SetLogger(logger)
	}
}

// SetLogger 设置日志记录器
func SetLogger(logger utils.Logger) {
	globalLogger = logger
}

// GetLogger 获取日志记录器
func GetLogger() utils.Logger {
	return globalLogger
}

// Debug 打印调试日志
func Debug(a ...interface{}) {
	globalLogger.Debug(a...)
}

// Debugf 打印调试模板日志
func Debugf(format string, a ...interface{}) {
	globalLogger.Debugf(format, a...)
}

// Info 打印信息日志
func Info(a ...interface{}) {
	globalLogger.Info(a...)
}

// Infof 打印信息模板日志
func Infof(format string, a ...interface{}) {
	globalLogger.Infof(format, a...)
}

// Warn 打印警告日志
func Warn(a ...interface{}) {
	globalLogger.Warn(a...)
}

// Warnf 打印警告模板日志
func Warnf(format string, a ...interface{}) {
	globalLogger.Warnf(format, a...)
}

// Error 打印错误日志
func Error(a ...interface{}) {
	globalLogger.Error(a...)
}

// Errorf 打印错误模板日志
func Errorf(format string, a ...interface{}) {
	globalLogger.Errorf(format, a...)
}

// Fatal 打印致命错误日志
func Fatal(a ...interface{}) {
	globalLogger.Fatal(a...)
	os.Exit(1)
}

// Fatalf 打印致命错误模板日志
func Fatalf(format string, a ...interface{}) {
	globalLogger.Fatalf(format, a...)
	os.Exit(1)
}

// Panic 打印Panic日志
func Panic(a ...interface{}) {
	globalLogger.Panic(a...)
}

// Panicf 打印Panic模板日志
func Panicf(format string, a ...interface{}) {
	globalLogger.Panicf(format, a...)
}

func buildErr(a ...interface{}) error {
	var msg string
	if c := len(a); c > 0 {
		msg = fmt.Sprintf(strings.TrimSuffix(strings.Repeat("%v ", c), " "), a...)
	}

	msgt := strings.TrimSuffix(msg, "\n")

	errr := errors.NewError(a)
	err := fmt.Errorf("%s: %s", msgt, errr)

	return err
}

func buildErrf(format string, a ...interface{}) error {
	var msg string
	if c := len(a); c > 0 {
		msg = fmt.Sprintf(strings.TrimSuffix(strings.Repeat("%v ", c), " "), a...)
	}

	msgt := strings.TrimSuffix(msg, "\n")

	errr := errors.NewError(a)
	err := fmt.Errorf("%s, %s: %s", format, msgt, errr)

	return err
}

const (
	loggerLevelKey             = "log.level"
	remoteAsyncOutputCallerKey = "log.zap.outputCaller"
)

type LoggerConfigChangeListener struct {
	defaultLogger utils.Logger
}
