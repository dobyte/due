package log

import (
	"github.com/dobyte/due/v2/log/internal"
)

// Terminal is a log output terminal.
type Terminal string

const (
	TerminalConsole Terminal = "console" // Console terminal
	TerminalFile    Terminal = "file"    // File terminal
)

type (
	Level  = internal.Level
	Entity = internal.Entity
	Syncer = internal.Syncer
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
