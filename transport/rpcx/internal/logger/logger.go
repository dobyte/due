package logger

import (
	"sync"

	"github.com/dobyte/due/v2/log"
	rpcxlog "github.com/smallnest/rpcx/log"
)

var once sync.Once

// InitLogger 初始化 rpcx 日志
// 将 rpcx 内部日志桥接到 due 日志框架，仅记录错误级别及以上日志（幂等）
func InitLogger() {
	once.Do(func() {
		rpcxlog.SetLogger(&logger{
			level:  log.LevelError,
			logger: log.GetLogger(),
		})
	})
}

// logger rpcx 日志适配器，实现 rpcxlog.Logger 接口
type logger struct {
	level  log.Level
	logger log.Logger
}

// Debug 记录调试日志
// @param v ...any 日志内容
func (l *logger) Debug(v ...any) {
	if l.level <= log.LevelDebug {
		l.logger.Print(log.LevelDebug, v...)
	}
}

// Debugf 记录格式化调试日志
// @param format string 日志格式
// @param v ...any 日志参数
func (l *logger) Debugf(format string, v ...any) {
	if l.level <= log.LevelDebug {
		l.logger.Printf(log.LevelDebug, format, v...)
	}
}

// Info 记录信息日志
// @param v ...any 日志内容
func (l *logger) Info(v ...any) {
	if l.level <= log.LevelInfo {
		l.logger.Print(log.LevelInfo, v...)
	}
}

// Infof 记录格式化信息日志
// @param format string 日志格式
// @param v ...any 日志参数
func (l *logger) Infof(format string, v ...any) {
	if l.level <= log.LevelInfo {
		l.logger.Printf(log.LevelInfo, format, v...)
	}
}

// Warn 记录警告日志
// @param v ...any 日志内容
func (l *logger) Warn(v ...any) {
	if l.level <= log.LevelWarn {
		l.logger.Print(log.LevelWarn, v...)
	}
}

// Warnf 记录格式化警告日志
// @param format string 日志格式
// @param v ...any 日志参数
func (l *logger) Warnf(format string, v ...any) {
	if l.level <= log.LevelWarn {
		l.logger.Printf(log.LevelWarn, format, v...)
	}
}

// Error 记录错误日志
// @param v ...any 日志内容
func (l *logger) Error(v ...any) {
	if l.level <= log.LevelError {
		l.logger.Print(log.LevelError, v...)
	}
}

// Errorf 记录格式化错误日志
// @param format string 日志格式
// @param v ...any 日志参数
func (l *logger) Errorf(format string, v ...any) {
	if l.level <= log.LevelError {
		l.logger.Printf(log.LevelError, format, v...)
	}
}

// Fatal 记录致命日志
// @param v ...any 日志内容
func (l *logger) Fatal(v ...any) {
	if l.level <= log.LevelFatal {
		l.logger.Print(log.LevelFatal, v...)
	}
}

// Fatalf 记录格式化致命日志
// @param format string 日志格式
// @param v ...any 日志参数
func (l *logger) Fatalf(format string, v ...any) {
	if l.level <= log.LevelFatal {
		l.logger.Printf(log.LevelFatal, format, v...)
	}
}

// Panic 记录恐慌日志
// @param v ...any 日志内容
func (l *logger) Panic(v ...any) {
	if l.level <= log.LevelPanic {
		l.logger.Print(log.LevelPanic, v...)
	}
}

// Panicf 记录格式化恐慌日志
// @param format string 日志格式
// @param v ...any 日志参数
func (l *logger) Panicf(format string, v ...any) {
	if l.level <= log.LevelPanic {
		l.logger.Printf(log.LevelPanic, format, v...)
	}
}
