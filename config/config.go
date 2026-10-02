package config

import (
	"context"

	"github.com/dobyte/due/v2/core/value"
)

var globalConfigurator Configurator

// SetConfigurator sets the global configurator, closing the previous one when it is not nil.
func SetConfigurator(configurator Configurator) {
	if globalConfigurator != nil {
		globalConfigurator.Close()
	}
	globalConfigurator = configurator
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
	if globalConfigurator == nil {
		return false
	}

	return globalConfigurator.Has(pattern)
}

// Get returns the config value under pattern, falling back to def when the config is missing.
func Get(pattern string, def ...any) value.Value {
	if globalConfigurator == nil {
		return value.NewValue()
	}

	return globalConfigurator.Get(pattern, def...)
}

// Set sets the config value under pattern.
func Set(pattern string, value any) error {
	if globalConfigurator == nil {
		return nil
	}

	return globalConfigurator.Set(pattern, value)
}

// Match returns a [Matcher] over the given patterns.
func Match(patterns ...string) Matcher {
	if globalConfigurator == nil {
		return newEmptyMatcher()
	}

	return globalConfigurator.Match(patterns...)
}

// Watch registers cb to be called when one of the named configs changes; empty names means every
// config.
func Watch(cb WatchCallbackFunc, names ...string) {
	if globalConfigurator == nil {
		return
	}

	globalConfigurator.Watch(cb, names...)
}

// Load loads the configurations of the named source; empty file means every file of the source.
func Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	if globalConfigurator == nil {
		return nil, nil
	}

	return globalConfigurator.Load(ctx, source, file...)
}

// Store saves content as file under the named source, merging it with the existing config unless
// override is true.
func Store(ctx context.Context, source string, file string, content any, override ...bool) error {
	if globalConfigurator == nil {
		return nil
	}

	return globalConfigurator.Store(ctx, source, file, content, override...)
}

// Close closes the global configurator and stops watching config changes.
func Close() {
	if globalConfigurator != nil {
		globalConfigurator.Close()
	}
}
