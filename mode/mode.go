package mode

import (
	"github.com/dobyte/due/v2/env"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/flag"
)

const (
	dueModeEtcName = "etc.mode" // 配置文件中的模式键名
	dueModeArgName = "mode"     // 运行参数中的模式项名
	dueModeEnvName = "DUE_MODE" // 环境变量中的模式键名
)

const (
	// DebugMode 调试模式
	DebugMode = "debug"
	// TestMode 测试模式
	TestMode = "test"
	// PreReleaseMode 预发布模式
	PreReleaseMode = "pre-release"
	// ReleaseMode 发布模式
	ReleaseMode = "release"
)

var dueMode string

// init 初始化运行模式
// 按优先级从配置来源读取运行模式并设置；优先级：配置文件 < 环境变量 < 运行参数 < mode.SetMode()
func init() {
	mode := etc.Get(dueModeEtcName, DebugMode).String()
	mode = env.Get(dueModeEnvName, mode).String()
	mode = flag.String(dueModeArgName, mode)
	SetMode(mode)
}

// SetMode 设置运行模式
// 模式需为debug、test、pre-release或release之一，空值按debug处理
// @param m string 待设置的运行模式
func SetMode(m string) {
	if m == "" {
		m = DebugMode
	}

	switch m {
	case DebugMode, TestMode, PreReleaseMode, ReleaseMode:
		dueMode = m
	default:
		panic("due mode unknown: " + m + " (available mode: debug test pre-release release)")
	}
}

// GetMode 获取运行模式
// @return @1 string 当前运行模式
func GetMode() string {
	return dueMode
}

// IsDebugMode 是否Debug模式
// @return @1 bool 当前是否为Debug模式
func IsDebugMode() bool {
	return dueMode == DebugMode
}

// IsTestMode 是否Test模式
// @return @1 bool 当前是否为Test模式
func IsTestMode() bool {
	return dueMode == TestMode
}

// IsPreReleaseMode 是否PreRelease模式
// @return @1 bool 当前是否为PreRelease模式
func IsPreReleaseMode() bool {
	return dueMode == PreReleaseMode
}

// IsReleaseMode 是否Release模式
// @return @1 bool 当前是否为Release模式
func IsReleaseMode() bool {
	return dueMode == ReleaseMode
}
