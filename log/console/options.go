package console

import (
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultFormat = FormatText
)

const (
	defaultFormatKey = "etc.log.console.format"
)

// Option configures the syncer.
type Option func(o *options)

type options struct {
	format Format // Output format
}

func defaultOptions() *options {
	return &options{
		format: Format(etc.Get(defaultFormatKey, defaultFormat).String()),
	}
}

// WithFormat sets the output format.
func WithFormat(format Format) Option {
	return func(o *options) { o.format = format }
}
