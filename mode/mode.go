package mode

import (
	"github.com/dobyte/due/v2/env"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/flag"
)

const (
	dueModeEtcName = "etc.mode"
	dueModeArgName = "mode"
	dueModeEnvName = "DUE_MODE"
)

const (
	// DebugMode indicates due mode is debug.
	DebugMode = "debug"
	// ReleaseMode indicates due mode is release.
	ReleaseMode = "release"
	// TestMode indicates due mode is test.
	TestMode = "test"
)

var dueMode string

// 优先级： env < etc < arg
func init() {
	mode := env.Get(dueModeEnvName, DebugMode).String()
	mode = etc.Get(dueModeEtcName, mode).String()
	mode = flag.String(dueModeArgName, mode)
	SetMode(mode)
}

// SetMode 设置运行模式
func SetMode(m string) {
	if m == "" {
		m = DebugMode
	}

	switch m {
	case DebugMode, TestMode, ReleaseMode:
		dueMode = m
	default:
		panic("due mode unknown: " + m + " (available mode: debug test release)")
	}
}

// GetMode 获取运行模式
func GetMode() string {
	return dueMode
}

// IsDebugMode 是否Debug模式
func IsDebugMode() bool {
	return dueMode == DebugMode
}

// IsTestMode 是否Test模式
func IsTestMode() bool {
	return dueMode == TestMode
}

// IsReleaseMode 是否Release模式
func IsReleaseMode() bool {
	return dueMode == ReleaseMode
}
