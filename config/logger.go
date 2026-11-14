package config

import (
	"log"

	"github.com/dobyte/due/v2/core/config"
	internallog "github.com/dobyte/due/v2/internal/logger"
)

// logger is a stateless marker that reports through the framework logger when it is registered by
// the log package, and falls back to the standard logger otherwise. The framework logger is
// resolved for every record instead of being captured at construction, so a logger that is not set
// yet does not become a permanently nil logger.
type logger struct{}

func (logger) Infof(format string, a ...any) {
	if l := internallog.Framework(); l != nil {
		l.Infof(format, a...)

		return
	}

	log.Printf(format, a...)
}

func (logger) Warnf(format string, a ...any) {
	if l := internallog.Framework(); l != nil {
		l.Warnf(format, a...)

		return
	}

	log.Printf(format, a...)
}

func (logger) Errorf(format string, a ...any) {
	if l := internallog.Framework(); l != nil {
		l.Errorf(format, a...)

		return
	}

	log.Printf(format, a...)
}

var _ config.Logger = logger{}
