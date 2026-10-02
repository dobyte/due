// Package etc holds the project bootstrap configuration; it is commonly used for cluster
// configuration, service-component configuration and the like.
//
// etc can only be configured through a configuration file and cannot be modified through the
// master management server. To use configuration in business code, prefer the config
// configuration center, whose data can be modified dynamically through the master management
// server.
package etc

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/config/file/core"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/env"
	"github.com/dobyte/due/v2/flag"
)

const (
	dueEtcEnvName  = "DUE_ETC"
	dueEtcArgName  = "etc"
	defaultEtcPath = "./etc"
)

var globalConfigurator config.Configurator

func init() {
	path := env.Get(dueEtcEnvName, defaultEtcPath).String()
	path = flag.String(dueEtcArgName, path)
	globalConfigurator = config.NewConfigurator(config.WithSources(core.NewSource(path, config.ReadOnly)))
}

// SetConfigurator sets the configurator.
func SetConfigurator(configurator config.Configurator) {
	if globalConfigurator != nil {
		globalConfigurator.Close()
	}

	globalConfigurator = configurator
}

// GetConfigurator returns the configurator.
func GetConfigurator() config.Configurator {
	return globalConfigurator
}

// Has reports whether the configuration matching pattern exists.
func Has(pattern string) bool {
	return globalConfigurator.Has(pattern)
}

// Get returns the configuration value matching pattern, falling back to def when it is absent.
func Get(pattern string, def ...any) value.Value {
	return globalConfigurator.Get(pattern, def...)
}

// Set sets the configuration value for pattern.
func Set(pattern string, value any) error {
	return globalConfigurator.Set(pattern, value)
}

// Match matches several patterns and returns a matcher.
func Match(patterns ...string) config.Matcher {
	return globalConfigurator.Match(patterns...)
}

// Close closes configuration watching.
func Close() {
	globalConfigurator.Close()
}
