package log

var globalLogger Logger

func init() {
	SetLogger(NewLogger(WithCallerSkip(2)))
}

// SetLogger 设置日志记录器
func SetLogger(logger Logger) {
	if logger == nil {
		return
	}

	globalLogger = logger
}

// GetLogger 获取日志记录器
func GetLogger() Logger {
	return globalLogger
}

// Print 打印日志，不含堆栈信息
func Print(level Level, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Print(level, a...)
	}
}

// Printf 打印模板日志，不含堆栈信息
func Printf(level Level, format string, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Printf(level, format, a...)
	}
}

// Debug 打印调试日志
func Debug(a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(a...)
	}
}

// Debugf 打印调试模板日志
func Debugf(format string, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debugf(format, a...)
	}
}

// Info 打印信息日志
func Info(a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(a...)
	}
}

// Infof 打印信息模板日志
func Infof(format string, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Infof(format, a...)
	}
}

// Warn 打印警告日志
func Warn(a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(a...)
	}
}

// Warnf 打印警告模板日志
func Warnf(format string, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warnf(format, a...)
	}
}

// Error 打印错误日志
func Error(a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(a...)
	}
}

// Errorf 打印错误模板日志
func Errorf(format string, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Errorf(format, a...)
	}
}

// Fatal 打印致命错误日志
func Fatal(a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Fatal(a...)
	}
}

// Fatalf 打印致命错误模板日志
func Fatalf(format string, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Fatalf(format, a...)
	}
}

// Panic 打印Panic日志
func Panic(a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Panic(a...)
	}
}

// Panicf 打印Panic模板日志
func Panicf(format string, a ...interface{}) {
	if globalLogger != nil {
		globalLogger.Panicf(format, a...)
	}
}

// Close 关闭日志
func Close() {
	if globalLogger != nil {
		_ = globalLogger.Close()
	}
}
