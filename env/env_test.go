package env_test

import (
	"testing"

	"github.com/dobyte/due/v2/env"
)

// TestGet verifies Get returns the environment value when the key is set and the fallback
// otherwise.
func TestGet(t *testing.T) {
	const absentKey = "DUE_TEST_ENV_UNSET_KEY"

	t.Run("unset with default", func(t *testing.T) {
		if got := env.Get(absentKey, "fallback").String(); got != "fallback" {
			t.Errorf("Get() = %q, want %q", got, "fallback")
		}
	})

	t.Run("unset without default", func(t *testing.T) {
		if got := env.Get(absentKey).String(); got != "" {
			t.Errorf("Get() = %q, want empty", got)
		}
	})

	t.Run("set", func(t *testing.T) {
		const key = "DUE_TEST_ENV_SET_KEY"
		t.Setenv(key, "value")

		if got := env.Get(key).String(); got != "value" {
			t.Errorf("Get() = %q, want %q", got, "value")
		}
	})
}

// TestSetDelHas verifies Set, Del and Has manipulate the environment consistently.
func TestSetDelHas(t *testing.T) {
	const key = "DUE_TEST_ENV_MUTABLE_KEY"
	t.Cleanup(func() { _ = env.Del(key) })

	if err := env.Set(key, "1"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !env.Has(key) {
		t.Errorf("Has() = false, want true")
	}
	if got := env.Get(key).String(); got != "1" {
		t.Errorf("Get() = %q, want %q", got, "1")
	}

	if err := env.Del(key); err != nil {
		t.Fatalf("Del() error = %v", err)
	}
	if env.Has(key) {
		t.Errorf("Has() = true, want false")
	}
}
