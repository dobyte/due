package pprof

import (
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultAddr = ":0" // Listen address
)

const (
	defaultAddrKey = "etc.pprof.addr"
)

type Option func(o *options)

type options struct {
	addr string // Listen address
}

func defaultOptions() *options {
	opts := &options{
		addr: defaultAddr,
	}

	if addr := etc.Get(defaultAddrKey).String(); addr != "" {
		opts.addr = addr
	}

	return opts
}

// WithAddr sets the listen address.
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}
