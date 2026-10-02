package env

import (
	"os"

	"github.com/dobyte/due/v2/core/value"
)

// Get returns the value of the environment variable key, falling back to def when it is unset.
func Get(key string, def ...any) value.Value {
	if val, ok := os.LookupEnv(key); ok {
		return value.NewValue(val)
	}

	return value.NewValue(def...)
}

// Set sets the environment variable key to value.
func Set(key string, value string) error {
	return os.Setenv(key, value)
}

// Del removes the environment variable key.
func Del(key string) error {
	return os.Unsetenv(key)
}

// Has reports whether the environment variable key exists.
func Has(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}
