package internal

const (
	red    = "31"
	yellow = "33"
	blue   = "36"
	gray   = "37"
)

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

// Color 获取日志级别颜色
func (l Level) Color() string {
	switch l {
	case LevelDebug:
		return gray
	case LevelWarn:
		return yellow
	case LevelError, LevelFatal, LevelPanic:
		return red
	default:
		return blue
	}
}

func (l Level) Label() string {
	switch l {
	case LevelDebug:
		return "DEBU"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERRO"
	case LevelFatal:
		return "FATA"
	case LevelPanic:
		return "PANI"
	default:
		return "NONE"
	}
}
