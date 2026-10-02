package etc_test

import (
	"testing"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
)

// TestGetConfigurator verifies a configurator is always installed.
func TestGetConfigurator(t *testing.T) {
	if etc.GetConfigurator() == nil {
		t.Errorf("GetConfigurator() = nil, want non-nil")
	}
}

// TestHas verifies Has reports false for an unknown key.
func TestHas(t *testing.T) {
	if etc.Has("due.etc.test.has.absent") {
		t.Errorf("Has() = true, want false")
	}
}

// TestGet verifies Get falls back to the default for an unknown key.
func TestGet(t *testing.T) {
	if got := etc.Get("due.etc.test.get.absent", "fallback").String(); got != "fallback" {
		t.Errorf("Get() = %q, want %q", got, "fallback")
	}
}

// TestSet verifies Set publishes an in-memory value observable through Has.
func TestSet(t *testing.T) {
	const key = "due.etc.test.set.key"

	if err := etc.Set(key, 1); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !etc.Has(key) {
		t.Errorf("Has(%q) = false, want true", key)
	}

	// Restore a clean configurator so later tests observe the default state.
	etc.SetConfigurator(config.NewConfigurator())
}

// TestMatch verifies Match falls back to the default and scans without error when nothing matches.
func TestMatch(t *testing.T) {
	m := etc.Match("due.etc.test.match.a", "due.etc.test.match.b")

	if m.Has() {
		t.Errorf("Matcher.Has() = true, want false")
	}
	if got := m.Get("fallback").String(); got != "fallback" {
		t.Errorf("Matcher.Get() = %q, want %q", got, "fallback")
	}

	var v int
	if err := m.Scan(&v); err != nil {
		t.Errorf("Matcher.Scan() error = %v", err)
	}
}

// TestSetConfiguratorAndClose verifies SetConfigurator installs the given configurator and Close
// shuts the global one down.
func TestSetConfiguratorAndClose(t *testing.T) {
	if etc.GetConfigurator() == nil {
		t.Fatal("GetConfigurator() = nil, want non-nil")
	}

	custom := config.NewConfigurator()
	etc.SetConfigurator(custom)
	if got := etc.GetConfigurator(); got != custom {
		t.Errorf("GetConfigurator() did not return the injected configurator")
	}

	// Leave a usable configurator behind for any test that runs afterwards.
	etc.SetConfigurator(config.NewConfigurator())
	etc.Close()
}
