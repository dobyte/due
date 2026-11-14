package config

import (
	"context"

	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
)

// globalConfigurator is installed during initialization and is not modified afterwards, so the
// global entry points read it without synchronization.
var globalConfigurator Configurator

// SetConfigurator sets the global configurator, closing the previous one when it is not nil. It
// must be called during initialization, before the configurator is used concurrently.
func SetConfigurator(configurator Configurator) {
	previous := globalConfigurator
	globalConfigurator = configurator

	if previous != nil && previous != configurator {
		previous.Close()
	}
}

// GetConfigurator returns the global configurator.
func GetConfigurator() Configurator {
	return globalConfigurator
}

// SetConfiguratorWithSources creates a configurator from sources and installs it as the global
// configurator.
func SetConfiguratorWithSources(sources ...Source) {
	SetConfigurator(NewConfigurator(WithSources(sources...)))
}

// Has reports whether a config matching pattern exists.
func Has(pattern string) bool {
	if c := GetConfigurator(); c == nil {
		return false
	} else {
		return c.Has(pattern)
	}
}

// Get returns the config value under pattern, falling back to def when the config is missing. The
// returned value must be treated as read-only, since it may reference the internal config maps and
// slices.
func Get(pattern string, def ...any) value.Value {
	if c := GetConfigurator(); c == nil {
		return value.NewValue()
	} else {
		return c.Get(pattern, def...)
	}
}

// Set sets the config value under pattern.
func Set(pattern string, value any) error {
	if c := GetConfigurator(); c == nil {
		return errors.ErrNotFoundConfigurator
	} else {
		return c.Set(pattern, value)
	}
}

// Match returns a [Matcher] over the given patterns.
func Match(patterns ...string) Matcher {
	if c := GetConfigurator(); c == nil {
		return newEmptyMatcher()
	} else {
		return c.Match(patterns...)
	}
}

// Watch registers cb to be called when one of the named configs changes; empty names means every
// config.
func Watch(cb WatchCallbackFunc, names ...string) {
	if c := GetConfigurator(); c != nil {
		c.Watch(cb, names...)
	}
}

// Load loads the configurations of the named source; empty file means every file of the source. It
// reports [errors.ErrNotFoundConfigurator] when no global configurator is installed.
func Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	if c := GetConfigurator(); c == nil {
		return nil, errors.ErrNotFoundConfigurator
	} else {
		return c.Load(ctx, source, file...)
	}
}

// Store saves content as file under the named source, merging it with the existing config unless
// override is true. It reports [errors.ErrNotFoundConfigurator] when no global configurator is
// installed.
func Store(ctx context.Context, source string, file string, content any, override ...bool) error {
	if c := GetConfigurator(); c == nil {
		return errors.ErrNotFoundConfigurator
	} else {
		return c.Store(ctx, source, file, content, override...)
	}
}

// Close closes the global configurator and stops watching config changes. The global configurator
// is cleared, so later calls do not observe the closed configurator. Like [SetConfigurator], it is
// part of the initialization lifecycle and is not synchronized.
func Close() {
	c := globalConfigurator
	globalConfigurator = nil

	if c != nil {
		c.Close()
	}
}
