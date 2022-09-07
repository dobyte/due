/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/30 5:08 下午
 * @Desc: TODO
 */

package std

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

// 日志输出格式
type Format int

const (
	TextFormat Format = iota // 文本格式
	JsonFormat               // JSON格式
)

// 日志切割规则
type CutRule int

const (
	CutByYear   CutRule = iota + 1 // 按照年切割
	CutByMonth                     // 按照月切割
	CutByDay                       // 按照日切割
	CutByHour                      // 按照时切割
	CutByMinute                    // 按照分切割
	CutBySecond                    // 按照秒切割
)

// 调用则格式
type CallerFormat int

const (
	CallerShortPath CallerFormat = iota // 调用者短路径
	CallerFullPath                      // 调用者全路径
)
