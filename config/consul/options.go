package consul

import (
	"context"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	"github.com/hashicorp/consul/api"
)

const (
	defaultAddr = "127.0.0.1:8500"
	defaultPath = "config"
	defaultMode = config.ReadOnly
)

const (
	defaultAddrKey = "etc.config.consul.addr"
	defaultPathKey = "etc.config.consul.path"
	defaultModeKey = "etc.config.consul.mode"
)

// Option is a function that configures the [options].
type Option func(o *options)

// options holds the configuration options of a [Source].
type options struct {
	// ctx is the context and defaults to [context.Background].
	ctx context.Context

	// addr is the client connection address.
	// It configures the built-in client and defaults to 127.0.0.1:8500.
	addr string

	// client is the external client.
	// When it is provided, it takes precedence over the built-in client. It defaults to nil.
	client *api.Client

	// path is the path and defaults to /config.
	path string

	// mode is the read-write mode.
	// It supports the read-only, write-only and read-write modes and defaults to read-only.
	mode config.Mode
}

// defaultOptions creates the default options, reading each parameter from the
// configuration environment and filling in its default value.
func defaultOptions() *options {
	return &options{
		ctx:  context.Background(),
		addr: etc.Get(defaultAddrKey, defaultAddr).String(),
		path: etc.Get(defaultPathKey, defaultPath).String(),
		mode: config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
	}
}

// WithAddr sets the client connection address.
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithClient sets the external client.
func WithClient(client *api.Client) Option {
	return func(o *options) { o.client = client }
}

// WithContext sets the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithPath sets the base path.
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode sets the read-write mode.
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}
