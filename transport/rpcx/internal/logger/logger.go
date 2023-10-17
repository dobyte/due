package logger

import (
	"github.com/dobyte/due/v2/log"
	rpcxlog "github.com/smallnest/rpcx/log"
	"sync"
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

func (l *logger) Debug(v ...interface{}) {
	if l.level <= log.DebugLevel {
		l.logger.Print(log.DebugLevel, v...)
	}
}

func (l *logger) Debugf(format string, v ...interface{}) {
	if l.level <= log.DebugLevel {
		l.logger.Printf(log.DebugLevel, format, v...)
	}
}

func (l *logger) Info(v ...interface{}) {
	if l.level <= log.InfoLevel {
		l.logger.Print(log.InfoLevel, v...)
	}
}

func (l *logger) Infof(format string, v ...interface{}) {
	if l.level <= log.InfoLevel {
		l.logger.Printf(log.InfoLevel, format, v...)
	}
}

func (l *logger) Warn(v ...interface{}) {
	if l.level <= log.WarnLevel {
		l.logger.Print(log.WarnLevel, v...)
	}
}

func (l *logger) Warnf(format string, v ...interface{}) {
	if l.level <= log.WarnLevel {
		l.logger.Printf(log.WarnLevel, format, v...)
	}
}

func (l *logger) Error(v ...interface{}) {
	if l.level <= log.ErrorLevel {
		l.logger.Print(log.ErrorLevel, v...)
	}
}

func (l *logger) Errorf(format string, v ...interface{}) {
	if l.level <= log.ErrorLevel {
		l.logger.Printf(log.ErrorLevel, format, v...)
	}
}

func (l *logger) Fatal(v ...interface{}) {
	if l.level <= log.FatalLevel {
		l.logger.Print(log.FatalLevel, v...)
	}
}

func (l *logger) Fatalf(format string, v ...interface{}) {
	if l.level <= log.FatalLevel {
		l.logger.Printf(log.FatalLevel, format, v...)
	}
}

func (l *logger) Panic(v ...interface{}) {
	if l.level <= log.PanicLevel {
		l.logger.Print(log.PanicLevel, v...)
	}
}

func (l *logger) Panicf(format string, v ...interface{}) {
	if l.level <= log.PanicLevel {
		l.logger.Printf(log.PanicLevel, format, v...)
	}
}
