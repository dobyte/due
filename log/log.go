package log

var globalLogger Logger

func init() {
	SetLogger(NewLogger())
}

// SetLogger sets the global logger.
func SetLogger(logger Logger) {
	if logger == nil {
		return
	}

	if globalLogger != nil {
		globalLogger.Close()
	}

	globalLogger = logger
}

// GetLogger returns the global logger.
func GetLogger() Logger {
	return globalLogger
}

// Print writes a log record without stack information.
func Print(level Level, a ...any) {
	if globalLogger != nil {
		globalLogger.Print(level, a...)
	}
}

// Printf writes a formatted log record without stack information.
func Printf(level Level, format string, a ...any) {
	if globalLogger != nil {
		globalLogger.Printf(level, format, a...)
	}
}

// Debug writes a debug-level log record.
func Debug(a ...any) {
	if globalLogger != nil {
		globalLogger.Debug(a...)
	}
}

// Debugf writes a formatted debug-level log record.
func Debugf(format string, a ...any) {
	if globalLogger != nil {
		globalLogger.Debugf(format, a...)
	}
}

// Info writes an info-level log record.
func Info(a ...any) {
	if globalLogger != nil {
		globalLogger.Info(a...)
	}
}

// Infof writes a formatted info-level log record.
func Infof(format string, a ...any) {
	if globalLogger != nil {
		globalLogger.Infof(format, a...)
	}
}

// Warn writes a warn-level log record.
func Warn(a ...any) {
	if globalLogger != nil {
		globalLogger.Warn(a...)
	}
}

// Warnf writes a formatted warn-level log record.
func Warnf(format string, a ...any) {
	if globalLogger != nil {
		globalLogger.Warnf(format, a...)
	}
}

// Error writes an error-level log record.
func Error(a ...any) {
	if globalLogger != nil {
		globalLogger.Error(a...)
	}
}

// Errorf writes a formatted error-level log record.
func Errorf(format string, a ...any) {
	if globalLogger != nil {
		globalLogger.Errorf(format, a...)
	}
}

// Fatal writes a fatal-level log record.
func Fatal(a ...any) {
	if globalLogger != nil {
		globalLogger.Fatal(a...)
	}
}

// Fatalf writes a formatted fatal-level log record.
func Fatalf(format string, a ...any) {
	if globalLogger != nil {
		globalLogger.Fatalf(format, a...)
	}
}

// Panic writes a panic-level log record.
func Panic(a ...any) {
	if globalLogger != nil {
		globalLogger.Panic(a...)
	}
}

// Panicf writes a formatted panic-level log record.
func Panicf(format string, a ...any) {
	if globalLogger != nil {
		globalLogger.Panicf(format, a...)
	}
}

// Close closes the global logger.
func Close() {
	if globalLogger != nil {
		_ = globalLogger.Close()
	}
}
