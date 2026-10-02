package mode

import (
	"github.com/dobyte/due/v2/env"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/flag"
)

const (
	dueModeEtcName = "etc.mode" // Mode key in the configuration file
	dueModeArgName = "mode"     // Mode option in the command-line arguments
	dueModeEnvName = "DUE_MODE" // Mode key in the environment variables
)

const (
	// DebugMode is the debug mode.
	DebugMode = "debug"
	// TestMode is the test mode.
	TestMode = "test"
	// PreReleaseMode is the pre-release mode.
	PreReleaseMode = "pre-release"
	// ReleaseMode is the release mode.
	ReleaseMode = "release"
)

var dueMode string

// init initializes the run mode.
//
// The mode is read from each source in priority order: configuration file < environment variable <
// command-line argument < [SetMode].
func init() {
	mode := etc.Get(dueModeEtcName, DebugMode).String()
	mode = env.Get(dueModeEnvName, mode).String()
	mode = flag.String(dueModeArgName, mode)
	SetMode(mode)
}

// SetMode sets the run mode.
//
// The mode must be one of debug, test, pre-release or release; an empty value is treated as debug.
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

// GetMode returns the current run mode.
func GetMode() string {
	return dueMode
}

// IsDebugMode reports whether the run mode is debug.
func IsDebugMode() bool {
	return dueMode == DebugMode
}

// IsTestMode reports whether the run mode is test.
func IsTestMode() bool {
	return dueMode == TestMode
}

// IsPreReleaseMode reports whether the run mode is pre-release.
func IsPreReleaseMode() bool {
	return dueMode == PreReleaseMode
}

// IsReleaseMode reports whether the run mode is release.
func IsReleaseMode() bool {
	return dueMode == ReleaseMode
}
