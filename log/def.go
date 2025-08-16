package log

import (
	"github.com/dobyte/due/v2/log/internal"
)

// Terminal 日志输出终端
type Terminal string

const (
	TerminalConsole Terminal = "console" // 控制台
	TerminalFile    Terminal = "file"    // 文件
)

type (
	Level  = internal.Level
	Entity = internal.Entity
)

const (
	LevelNone  = internal.LevelNone
	LevelDebug = internal.LevelDebug
	LevelInfo  = internal.LevelInfo
	LevelWarn  = internal.LevelWarn
	LevelError = internal.LevelError
	LevelFatal = internal.LevelFatal
	LevelPanic = internal.LevelPanic
)
