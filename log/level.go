/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/8 10:37 上午
 * @Desc: TODO
 */

package log

// 日志级别
type Level int

const (
	DebugLevel Level = iota + 1 // DEBUG
	InfoLevel                   // INFO
	WarnLevel                   // WARN
	ErrorLevel                  // ERROR
	FatalLevel                  // FATAL
	PanicLevel                  // PANIC
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	case PanicLevel:
		return "PANIC"
	}
	return "UNKNOWN"
}
