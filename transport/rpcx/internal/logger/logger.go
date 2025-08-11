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
			level:  log.ErrorLevel,
			logger: log.GetLogger(),
		})
	})
}

type logger struct {
	level  log.Level
	logger log.Logger
}

func (l *logger) Debug(v ...any) {
	if l.level <= log.DebugLevel {
		l.logger.Print(log.DebugLevel, v...)
	}
}

func (l *logger) Debugf(format string, v ...any) {
	if l.level <= log.DebugLevel {
		l.logger.Printf(log.DebugLevel, format, v...)
	}
}

func (l *logger) Info(v ...any) {
	if l.level <= log.InfoLevel {
		l.logger.Print(log.InfoLevel, v...)
	}
}

func (l *logger) Infof(format string, v ...any) {
	if l.level <= log.InfoLevel {
		l.logger.Printf(log.InfoLevel, format, v...)
	}
}

func (l *logger) Warn(v ...any) {
	if l.level <= log.WarnLevel {
		l.logger.Print(log.WarnLevel, v...)
	}
}

func (l *logger) Warnf(format string, v ...any) {
	if l.level <= log.WarnLevel {
		l.logger.Printf(log.WarnLevel, format, v...)
	}
}

func (l *logger) Error(v ...any) {
	if l.level <= log.ErrorLevel {
		l.logger.Print(log.ErrorLevel, v...)
	}
}

func (l *logger) Errorf(format string, v ...any) {
	if l.level <= log.ErrorLevel {
		l.logger.Printf(log.ErrorLevel, format, v...)
	}
}

func (l *logger) Fatal(v ...any) {
	if l.level <= log.FatalLevel {
		l.logger.Print(log.FatalLevel, v...)
	}
}

func (l *logger) Fatalf(format string, v ...any) {
	if l.level <= log.FatalLevel {
		l.logger.Printf(log.FatalLevel, format, v...)
	}
}

func (l *logger) Panic(v ...any) {
	if l.level <= log.PanicLevel {
		l.logger.Print(log.PanicLevel, v...)
	}
}

func (l *logger) Panicf(format string, v ...any) {
	if l.level <= log.PanicLevel {
		l.logger.Printf(log.PanicLevel, format, v...)
	}
}
