package utils

type Logger interface {
	// Debug 打印调试日志
	Debug(a ...interface{})
	// Debugf 打印调试模板日志
	Debugf(format string, a ...interface{})
	// Info 打印信息日志
	Info(a ...interface{})
	// Infof 打印信息模板日志
	Infof(format string, a ...interface{})
	// Warn 打印警告日志
	Warn(a ...interface{})
	// Warnf 打印警告模板日志
	Warnf(format string, a ...interface{})
	// Error 打印错误日志
	Error(a ...interface{})
	// Errorf 打印错误模板日志
	Errorf(format string, a ...interface{})
	// Fatal 打印致命错误日志
	Fatal(a ...interface{})
	// Fatalf 打印致命错误模板日志
	Fatalf(format string, a ...interface{})
	// Panic 打印Panic日志
	Panic(a ...interface{})
	// Panicf 打印Panic模板日志
	Panicf(format string, a ...interface{})
}
