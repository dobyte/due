package config

import (
	"context"

	"github.com/dobyte/due/v2/core/config"
	"github.com/dobyte/due/v2/core/value"
)

// SetConfigurator sets the global configurator, closing the previous one when it is not nil. It
// must be called during initialization, before the configurator is used concurrently.
func SetConfigurator(configurator Configurator) {
	config.SetConfigurator(configurator)
}

// GetConfigurator returns the global configurator.
func GetConfigurator() Configurator {
	return config.GetConfigurator()
}

// SetConfiguratorWithSources creates a configurator with [NewConfigurator] and installs it as the
// global configurator. It therefore uses the same framework logger default as [NewConfigurator].
func SetConfiguratorWithSources(sources ...Source) {
	SetConfigurator(NewConfigurator(WithSources(sources...)))
}

// Has reports whether a config matching pattern exists.
func Has(pattern string) bool {
	return config.Has(pattern)
}

// Get returns the config value under pattern, falling back to def when the config is missing. The
// returned value must be treated as read-only, since it may reference the internal config maps and
// slices.
func Get(pattern string, def ...any) value.Value {
	return config.Get(pattern, def...)
}

// Set sets the config value under pattern.
func Set(pattern string, value any) error {
	return config.Set(pattern, value)
}

// Match returns a [Matcher] over the given patterns.
func Match(patterns ...string) Matcher {
	return config.Match(patterns...)
}

// Watch registers cb to be called when one of the named configs changes; empty names means every
// config.
func Watch(cb WatchCallbackFunc, names ...string) {
	config.Watch(cb, names...)
}

// Load loads the configurations of the named source; empty file means every file of the source. It
// reports errors.ErrNotFoundConfigurator when no global configurator is installed.
func Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	return config.Load(ctx, source, file...)
}

// Store saves content as file under the named source, merging it with the existing config unless
// override is true. It reports errors.ErrNotFoundConfigurator when no global configurator is
// installed.
func Store(ctx context.Context, source string, file string, content any, override ...bool) error {
	return config.Store(ctx, source, file, content, override...)
}

// Close closes the global configurator and clears it.
func Close() {
	config.Close()
}
