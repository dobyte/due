package config

import "github.com/dobyte/due/v2/core/config"

// Configurator manages a set of config sources and exposes the config loaded from them.
type Configurator = config.Configurator

// WatchCallbackFunc is called with the names of the configs that changed.
type WatchCallbackFunc = config.WatchCallbackFunc

// NewConfigurator returns a new [Configurator] configured with opts. The framework logger is used
// when the log package is initialized; otherwise the standard logger is used.
func NewConfigurator(opts ...Option) Configurator {
	opts = append([]Option{config.WithLogger(logger{})}, opts...)

	return config.NewConfigurator(opts...)
}
