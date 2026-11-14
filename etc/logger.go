package etc

import "log"

// stdLogger writes every record to the standard library logger.
type stdLogger struct{}

func (stdLogger) Infof(format string, a ...any)  { log.Printf(format, a...) }
func (stdLogger) Warnf(format string, a ...any)  { log.Printf(format, a...) }
func (stdLogger) Errorf(format string, a ...any) { log.Printf(format, a...) }
