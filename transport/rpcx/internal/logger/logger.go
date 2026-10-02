package logger

import (
	"sync"

	"github.com/dobyte/due/v2/log"
	rpcxlog "github.com/smallnest/rpcx/log"
)

var once sync.Once

// InitLogger initializes the rpcx logger.
//
// It bridges the internal rpcx log to the due logging framework and records only error level and
// above. It is idempotent.
func InitLogger() {
	once.Do(func() {
		rpcxlog.SetLogger(&logger{
			level:  log.LevelError,
			logger: log.GetLogger(),
		})
	})
}

// logger is the rpcx log adapter that implements the rpcxlog.Logger interface.
type logger struct {
	level  log.Level
	logger log.Logger
}

// Debug logs a debug message.
func (l *logger) Debug(v ...any) {
	if l.level <= log.LevelDebug {
		l.logger.Print(log.LevelDebug, v...)
	}
}

// Debugf logs a formatted debug message.
func (l *logger) Debugf(format string, v ...any) {
	if l.level <= log.LevelDebug {
		l.logger.Printf(log.LevelDebug, format, v...)
	}
}

// Info logs an info message.
func (l *logger) Info(v ...any) {
	if l.level <= log.LevelInfo {
		l.logger.Print(log.LevelInfo, v...)
	}
}

// Infof logs a formatted info message.
func (l *logger) Infof(format string, v ...any) {
	if l.level <= log.LevelInfo {
		l.logger.Printf(log.LevelInfo, format, v...)
	}
}

// Warn logs a warning message.
func (l *logger) Warn(v ...any) {
	if l.level <= log.LevelWarn {
		l.logger.Print(log.LevelWarn, v...)
	}
}

// Warnf logs a formatted warning message.
func (l *logger) Warnf(format string, v ...any) {
	if l.level <= log.LevelWarn {
		l.logger.Printf(log.LevelWarn, format, v...)
	}
}

// Error logs an error message.
func (l *logger) Error(v ...any) {
	if l.level <= log.LevelError {
		l.logger.Print(log.LevelError, v...)
	}
}

// Errorf logs a formatted error message.
func (l *logger) Errorf(format string, v ...any) {
	if l.level <= log.LevelError {
		l.logger.Printf(log.LevelError, format, v...)
	}
}

// Fatal logs a fatal message.
func (l *logger) Fatal(v ...any) {
	if l.level <= log.LevelFatal {
		l.logger.Print(log.LevelFatal, v...)
	}
}

// Fatalf logs a formatted fatal message.
func (l *logger) Fatalf(format string, v ...any) {
	if l.level <= log.LevelFatal {
		l.logger.Printf(log.LevelFatal, format, v...)
	}
}

// Panic logs a panic message.
func (l *logger) Panic(v ...any) {
	if l.level <= log.LevelPanic {
		l.logger.Print(log.LevelPanic, v...)
	}
}

// Panicf logs a formatted panic message.
func (l *logger) Panicf(format string, v ...any) {
	if l.level <= log.LevelPanic {
		l.logger.Printf(log.LevelPanic, format, v...)
	}
}
