package log

import "io"

type Option func(o *options)

type options struct {
	writer io.Writer
	prefix string
	flag   int
}

func WithWriter(writer io.Writer) Option { return func(o *options) { o.writer = writer } }

func WithPrefix(prefix string) Option { return func(o *options) { o.prefix = prefix } }

func WithFlag(flag int) Option { return func(o *options) { o.flag = flag } }
