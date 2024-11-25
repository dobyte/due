/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/8 10:37 上午
 * @Desc: TODO
 */

package log

import "strings"

// Level 日志级别
type Level int

const (
	NoneLevel  Level = iota // NONE
	DebugLevel              // DEBUG
	InfoLevel               // INFO
	WarnLevel               // WARN
	ErrorLevel              // ERROR
	FatalLevel              // FATAL
	PanicLevel              // PANIC
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
	default:
		return "PANIC"
	}
}

func ParseLevel(level string) Level {
	switch strings.ToUpper(level) {
	case DebugLevel.String():
		return DebugLevel
	case InfoLevel.String():
		return InfoLevel
	case WarnLevel.String():
		return WarnLevel
	case ErrorLevel.String():
		return ErrorLevel
	case FatalLevel.String():
		return FatalLevel
	case PanicLevel.String():
		return PanicLevel
	default:
		return NoneLevel
	}
}
