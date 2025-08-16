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

type Option func(o *options)

type options struct {
	format Format // 输出格式
}

func defaultOptions() *options {
	return &options{
		format: Format(etc.Get(defaultFormatKey, defaultFormat).String()),
	}
}

// WithFormat 设置输出格式
func WithFormat(format Format) Option {
	return func(o *options) { o.format = format }
}
