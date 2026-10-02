package file

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultPath = "./config"
	defaultMode = config.ReadOnly
)

const (
	defaultPathKey = "etc.config.file.path"
	defaultModeKey = "etc.config.file.mode"
)

// Option configures a [Source].
type Option func(o *options)

type options struct {
	// Path of the config file or config directory.
	path string

	// Read-write mode.
	// It supports read-only, write-only and read-write; the default is read-only.
	mode config.Mode
}

func defaultOptions() *options {
	return &options{
		path: etc.Get(defaultPathKey, defaultPath).String(),
		mode: config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
	}
}

// WithPath sets the path of the config file or config directory.
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode sets the read-write mode.
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}
