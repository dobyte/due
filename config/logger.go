package config

import (
	"github.com/dobyte/due/v2/core/config"
	"github.com/dobyte/due/v2/log"
)

// logger is a stateless marker that reports through the Due logger. The global logger is resolved
// for every record instead of being captured at construction, so a logger that is not set yet does
// not become a permanently nil logger.
type logger struct{}

func (logger) Infof(format string, a ...any)  { log.Infof(format, a...) }
func (logger) Warnf(format string, a ...any)  { log.Warnf(format, a...) }
func (logger) Errorf(format string, a ...any) { log.Errorf(format, a...) }

var _ config.Logger = logger{}
