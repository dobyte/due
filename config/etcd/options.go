package etcd

import (
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultAddr        = "127.0.0.1:2379"
	defaultDialTimeout = "5s"
	defaultPath        = "/config"
	defaultMode        = config.ReadOnly
	defaultTimeout     = "3s"
	defaultRetryTimes  = 3
)

const (
	defaultAddrsKey       = "etc.config.etcd.addrs"
	defaultDialTimeoutKey = "etc.config.etcd.dialTimeout"
	defaultPathKey        = "etc.config.etcd.path"
	defaultModeKey        = "etc.config.etcd.mode"
	defaultUsernameKey    = "etc.config.etcd.username"
	defaultPasswordKey    = "etc.config.etcd.password"
	defaultTimeoutKey     = "etc.config.etcd.timeout"
)

// Option is a function that configures the [options].
type Option func(o *options)

// options holds the configuration options of a [Source].
type options struct {
	// addrs is the client connection address.
	// It configures the built-in client and defaults to []string{"127.0.0.1:2379"}.
	addrs []string

	// dialTimeout is the dial timeout of the client.
	// It configures the built-in client and defaults to 5 seconds.
	dialTimeout time.Duration

	// client is the external client.
	// When it is provided, it takes precedence over the built-in client. It defaults to nil.
	client *clientv3.Client

	// path is the namespace path and defaults to /config.
	path string

	// mode is the read-write mode.
	// It supports the read-only, write-only and read-write modes and defaults to read-only.
	mode config.Mode

	// username is the username.
	username string

	// password is the password.
	password string

	// timeout is the context timeout and defaults to 3 seconds.
	timeout time.Duration
}

// defaultOptions creates the default options, reading each parameter from the
// configuration environment and filling in its default value.
func defaultOptions() *options {
	return &options{
		addrs:       etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		dialTimeout: etc.Get(defaultDialTimeoutKey, defaultDialTimeout).Duration(),
		path:        etc.Get(defaultPathKey, defaultPath).String(),
		mode:        config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
		username:    etc.Get(defaultUsernameKey).String(),
		password:    etc.Get(defaultPasswordKey).String(),
		timeout:     etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
	}
}

// WithAddrs sets the client connection addresses.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDialTimeout sets the dial timeout of the client.
func WithDialTimeout(dialTimeout time.Duration) Option {
	return func(o *options) { o.dialTimeout = dialTimeout }
}

// WithClient sets the external client.
func WithClient(client *clientv3.Client) Option {
	return func(o *options) { o.client = client }
}

// WithPath sets the namespace.
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode sets the read-write mode.
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}

// WithUsername sets the username.
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword sets the password.
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithTimeout sets the context timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}
