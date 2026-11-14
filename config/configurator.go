package config

import "github.com/dobyte/due/v2/core/config"

// Configurator manages a set of config sources and exposes the config loaded from them.
type Configurator = config.Configurator

// WatchCallbackFunc is called with the names of the configs that changed.
type WatchCallbackFunc = config.WatchCallbackFunc

// NewConfigurator returns a new [Configurator] configured with opts. It installs the framework
// logger when the log package is initialized and falls back to the standard logger otherwise.
// Unlike core/config.NewConfigurator, which uses the standard logger by default, this constructor
// targets the public config center.
func NewConfigurator(opts ...Option) Configurator {
	opts = append([]Option{config.WithLogger(logger{})}, opts...)

	return config.NewConfigurator(opts...)
}
