package logger

import (
	"sync"

	"github.com/dobyte/due/v2/log"
	rpcxlog "github.com/smallnest/rpcx/log"
)

var once sync.Once

func InitLogger() {
	once.Do(func() {
		rpcxlog.SetLogger(&logger{
			level:  log.LevelError,
			logger: log.GetLogger(),
		})
	})
}

type logger struct {
	level  log.Level
	logger log.Logger
}

func (l *logger) Debug(v ...any) {
	if l.level <= log.LevelDebug {
		l.logger.Print(log.LevelDebug, v...)
	}
}

func (l *logger) Debugf(format string, v ...any) {
	if l.level <= log.LevelDebug {
		l.logger.Printf(log.LevelDebug, format, v...)
	}
}

func (l *logger) Info(v ...any) {
	if l.level <= log.LevelInfo {
		l.logger.Print(log.LevelInfo, v...)
	}
}

func (l *logger) Infof(format string, v ...any) {
	if l.level <= log.LevelInfo {
		l.logger.Printf(log.LevelInfo, format, v...)
	}
}

func (l *logger) Warn(v ...any) {
	if l.level <= log.LevelWarn {
		l.logger.Print(log.LevelWarn, v...)
	}
}

func (l *logger) Warnf(format string, v ...any) {
	if l.level <= log.LevelWarn {
		l.logger.Printf(log.LevelWarn, format, v...)
	}
}

func (l *logger) Error(v ...any) {
	if l.level <= log.LevelError {
		l.logger.Print(log.LevelError, v...)
	}
}

func (l *logger) Errorf(format string, v ...any) {
	if l.level <= log.LevelError {
		l.logger.Printf(log.LevelError, format, v...)
	}
}

func (l *logger) Fatal(v ...any) {
	if l.level <= log.LevelFatal {
		l.logger.Print(log.LevelFatal, v...)
	}
}

func (l *logger) Fatalf(format string, v ...any) {
	if l.level <= log.LevelFatal {
		l.logger.Printf(log.LevelFatal, format, v...)
	}
}

func (l *logger) Panic(v ...any) {
	if l.level <= log.LevelPanic {
		l.logger.Print(log.LevelPanic, v...)
	}
}

func (l *logger) Panicf(format string, v ...any) {
	if l.level <= log.LevelPanic {
		l.logger.Printf(log.LevelPanic, format, v...)
	}
}
