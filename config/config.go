package config

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/core/value"
)

var (
	globalMu           sync.RWMutex
	globalConfigurator Configurator
)

// SetConfigurator sets the global configurator, closing the previous one when it is not nil.
func SetConfigurator(configurator Configurator) {
	globalMu.Lock()
	previous := globalConfigurator
	globalConfigurator = configurator
	globalMu.Unlock()

	if previous != nil && previous != configurator {
		previous.Close()
	}
}

// GetConfigurator returns the global configurator.
func GetConfigurator() Configurator {
	globalMu.RLock()
	defer globalMu.RUnlock()

	return globalConfigurator
}

// SetConfiguratorWithSources creates a configurator from sources and installs it as the global
// configurator.
func SetConfiguratorWithSources(sources ...Source) {
	SetConfigurator(NewConfigurator(WithSources(sources...)))
}

// Has reports whether a config matching pattern exists.
func Has(pattern string) bool {
	c := GetConfigurator()
	if c == nil {
		return false
	}

	return c.Has(pattern)
}

// Get returns the config value under pattern, falling back to def when the config is missing. The
// returned value must be treated as read-only, since it may reference the internal config maps and
// slices.
func Get(pattern string, def ...any) value.Value {
	c := GetConfigurator()
	if c == nil {
		return value.NewValue()
	}

	return c.Get(pattern, def...)
}

// Set sets the config value under pattern.
func Set(pattern string, value any) error {
	c := GetConfigurator()
	if c == nil {
		return nil
	}

	return c.Set(pattern, value)
}

// Match returns a [Matcher] over the given patterns.
func Match(patterns ...string) Matcher {
	c := GetConfigurator()
	if c == nil {
		return newEmptyMatcher()
	}

	return c.Match(patterns...)
}

// Watch registers cb to be called when one of the named configs changes; empty names means every
// config.
func Watch(cb WatchCallbackFunc, names ...string) {
	c := GetConfigurator()
	if c == nil {
		return
	}

	c.Watch(cb, names...)
}

// Load loads the configurations of the named source; empty file means every file of the source.
func Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	c := GetConfigurator()
	if c == nil {
		return nil, nil
	}

	return c.Load(ctx, source, file...)
}

// Store saves content as file under the named source, merging it with the existing config unless
// override is true.
func Store(ctx context.Context, source string, file string, content any, override ...bool) error {
	c := GetConfigurator()
	if c == nil {
		return nil
	}

	return c.Store(ctx, source, file, content, override...)
}

// Close closes the global configurator and stops watching config changes. The global configurator
// is cleared, so later calls do not observe the closed configurator.
func Close() {
	globalMu.Lock()
	c := globalConfigurator
	globalConfigurator = nil
	globalMu.Unlock()

	if c != nil {
		c.Close()
	}
}
