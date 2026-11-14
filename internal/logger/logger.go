// Package logger provides the minimal logging bridge shared by the framework logger and packages
// that cannot import the log package directly.
package logger

// Logger is the minimal logging interface used by the bridge.
type Logger interface {
	// Infof writes a formatted info-level log record.
	Infof(format string, a ...any)
	// Warnf writes a formatted warn-level log record.
	Warnf(format string, a ...any)
	// Errorf writes a formatted error-level log record.
	Errorf(format string, a ...any)
}

// provider resolves the framework logger. It is registered during initialization by the log
// package, so reads do not synchronize.
var provider func() Logger

// Register installs the framework logger provider during initialization.
func Register(p func() Logger) {
	provider = p
}

// Framework returns the registered framework logger, or nil when the log package did not register
// one.
func Framework() Logger {
	if provider == nil {
		return nil
	}

	return provider()
}
