package log

// Level 日志级别
type Level string

const (
	LevelNone  Level = "none"  // NONE
	LevelDebug Level = "debug" // DEBUG
	LevelInfo  Level = "info"  // INFO
	LevelWarn  Level = "warn"  // WARN
	LevelError Level = "error" // ERROR
	LevelFatal Level = "fatal" // FATAL
	LevelPanic Level = "panic" // PANIC
)

// Priority 获取日志级别优先级
func (l Level) Priority() int {
	switch l {
	case LevelDebug:
		return 1
	case LevelInfo:
		return 2
	case LevelWarn:
		return 3
	case LevelError:
		return 4
	case LevelFatal:
		return 5
	case LevelPanic:
		return 6
	default:
		return 0
	}
}

// Format 日志输出格式
type Format string

const (
	FormatText Format = "text" // 文本格式
	FormatJson Format = "json" // JSON格式
)

// Terminal 日志输出终端
type Terminal string

const (
	TerminalConsole Terminal = "console" // 控制台
	TerminalFile    Terminal = "file"    // 文件
)

// 日志翻转规则
type FileRotate string

const (
	FileRotateNone   FileRotate = "none"   // 不翻转
	FileRotateYear   FileRotate = "year"   // 按年翻转
	FileRotateMonth  FileRotate = "month"  // 按月翻转
	FileRotateWeek   FileRotate = "week"   // 按周翻转
	FileRotateDay    FileRotate = "day"    // 按天翻转
	FileRotateHour   FileRotate = "hour"   // 按时翻转
	FileRotateMinute FileRotate = "minute" // 按分翻转
	FileRotateSecond FileRotate = "second" // 按秒翻转
)
