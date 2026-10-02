package mode_test

import (
	"testing"

	"github.com/dobyte/due/v2/mode"
)

// TestSetModeAndPredicates verifies SetMode accepts every supported mode, defaults an empty value
// to debug, and updates the Is* predicates accordingly.
func TestSetModeAndPredicates(t *testing.T) {
	original := mode.GetMode()
	t.Cleanup(func() { mode.SetMode(original) })

	cases := []struct {
		name       string
		set        string
		want       string
		debug      bool
		test       bool
		preRelease bool
		release    bool
	}{
		{"empty defaults to debug", "", mode.DebugMode, true, false, false, false},
		{"debug", mode.DebugMode, mode.DebugMode, true, false, false, false},
		{"test", mode.TestMode, mode.TestMode, false, true, false, false},
		{"pre-release", mode.PreReleaseMode, mode.PreReleaseMode, false, false, true, false},
		{"release", mode.ReleaseMode, mode.ReleaseMode, false, false, false, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mode.SetMode(c.set)

			if got := mode.GetMode(); got != c.want {
				t.Errorf("GetMode() = %q, want %q", got, c.want)
			}
			if got := mode.IsDebugMode(); got != c.debug {
				t.Errorf("IsDebugMode() = %v, want %v", got, c.debug)
			}
			if got := mode.IsTestMode(); got != c.test {
				t.Errorf("IsTestMode() = %v, want %v", got, c.test)
			}
			if got := mode.IsPreReleaseMode(); got != c.preRelease {
				t.Errorf("IsPreReleaseMode() = %v, want %v", got, c.preRelease)
			}
			if got := mode.IsReleaseMode(); got != c.release {
				t.Errorf("IsReleaseMode() = %v, want %v", got, c.release)
			}
		})
	}
}

// TestSetModeUnknownPanics verifies SetMode panics on an unsupported mode.
func TestSetModeUnknownPanics(t *testing.T) {
	original := mode.GetMode()
	t.Cleanup(func() { mode.SetMode(original) })

	defer func() {
		if recover() == nil {
			t.Errorf("SetMode() did not panic for an unknown mode")
		}
	}()

	mode.SetMode("unknown-mode")
}
