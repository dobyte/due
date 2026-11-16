package etc

import "log"

// logger writes every record to the standard library logger.
type logger struct{}

func (logger) Infof(format string, a ...any)  { log.Printf(format, a...) }
func (logger) Warnf(format string, a ...any)  { log.Printf(format, a...) }
func (logger) Errorf(format string, a ...any) { log.Printf(format, a...) }
