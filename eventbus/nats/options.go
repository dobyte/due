package nats

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/nats-io/nats.go"
)

const (
	defaultUrl     = "nats://127.0.0.1:4222"
	defaultTimeout = 2 * time.Second
	defaultPrefix  = "due:eventbus"
)

const (
	defaultUrlKey     = "etc.eventbus.nats.url"
	defaultTimeoutKey = "etc.eventbus.nats.timeout"
	defaultPrefixKey  = "etc.eventbus.nats.prefix"
)

type Option func(o *options)

type options struct {
	// Client connection address.
	// Built-in client configuration, defaults to nats://127.0.0.1:4222.
	url string

	// Client connection timeout.
	// Built-in client configuration, defaults to 2s.
	timeout time.Duration

	// Client connection.
	// External client connection configuration; when set, the external client connection is
	// preferred. Defaults to nil.
	conn *nats.Conn

	// Prefix.
	// Key prefix, defaults to due:eventbus.
	prefix string
}

func defaultOptions() *options {
	return &options{
		url:     etc.Get(defaultUrlKey, defaultUrl).String(),
		timeout: etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		prefix:  etc.Get(defaultPrefixKey, defaultPrefix).String(),
	}
}

// WithUrl sets the connection address.
func WithUrl(url string) Option {
	return func(o *options) { o.url = url }
}

// WithTimeout sets the client connection timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithConn sets the external client connection.
func WithConn(conn *nats.Conn) Option {
	return func(o *options) { o.conn = conn }
}

// WithPrefix sets the prefix.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}
