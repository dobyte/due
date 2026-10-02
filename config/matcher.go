package config

import (
	"github.com/dobyte/due/v2/core/value"
)

// Matcher matches config values against an ordered list of patterns.
type Matcher interface {
	// Has reports whether a config exists for any of the patterns.
	Has() bool
	// Get returns the value of the first pattern that resolves, falling back to def when none does.
	Get(def ...any) value.Value
	// Scan scans the value of the first pattern that resolves into dest.
	Scan(dest any) error
}

type defaultMatcher struct {
	c        *defaultConfigurator
	patterns []string
}

func newEmptyMatcher() Matcher {
	return &defaultMatcher{}
}

// Has reports whether a config exists for any of the patterns.
func (m *defaultMatcher) Has() bool {
	if m.c == nil {
		return false
	}

	for _, pattern := range m.patterns {
		if ok := m.c.doHas(pattern); ok {
			return ok
		}
	}

	return false
}

// Get returns the value of the first pattern that resolves, falling back to def when none does.
func (m *defaultMatcher) Get(def ...any) value.Value {
	if m.c != nil {
		for _, pattern := range m.patterns {
			if val, ok := m.c.doGet(pattern); ok {
				return val
			}
		}
	}

	return value.NewValue(def...)
}

// Scan scans the value of the first pattern that resolves into dest.
func (m *defaultMatcher) Scan(dest any) error {
	if m.c != nil {
		for _, pattern := range m.patterns {
			if val, ok := m.c.doGet(pattern); ok {
				return val.Scan(dest)
			}
		}
	}

	return nil
}
