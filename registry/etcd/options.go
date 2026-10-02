package etcd

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultAddr        = "127.0.0.1:2379"
	defaultDialTimeout = "5s"
	defaultNamespace   = "services"
	defaultTimeout     = "3s"
	defaultLeaseTTL    = "15s"
)

const (
	defaultAddrsKey       = "etc.registry.etcd.addrs"
	defaultDialTimeoutKey = "etc.registry.etcd.dialTimeout"
	defaultNamespaceKey   = "etc.registry.etcd.namespace"
	defaultTimeoutKey     = "etc.registry.etcd.timeout"
	defaultUsernameKey    = "etc.registry.etcd.username"
	defaultPasswordKey    = "etc.registry.etcd.password"
	defaultLeaseTTLKey    = "etc.registry.etcd.leaseTTL"
)

// Option is a service registry and discovery option.
type Option func(o *options)

type options struct {
	// Client addresses.
	// Used by the built-in client and defaults to []string{"127.0.0.1:2379"}.
	addrs []string

	// Client dial timeout.
	// Used by the built-in client and defaults to 5 seconds.
	dialTimeout time.Duration

	// External client.
	// When an external client is provided it takes precedence over the built-in client and defaults to nil.
	client *clientv3.Client

	// Namespace.
	// Defaults to services.
	namespace string

	// Context timeout.
	// Defaults to 3 seconds.
	timeout time.Duration

	// Username.
	username string

	// Password.
	password string

	// Lease TTL.
	// Defaults to 15 seconds.
	leaseTTL time.Duration
}

func defaultOptions() *options {
	return &options{
		addrs:       etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		dialTimeout: etc.Get(defaultDialTimeoutKey, defaultDialTimeout).Duration(),
		namespace:   etc.Get(defaultNamespaceKey, defaultNamespace).String(),
		timeout:     etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		username:    etc.Get(defaultUsernameKey).String(),
		password:    etc.Get(defaultPasswordKey).String(),
		leaseTTL:    etc.Get(defaultLeaseTTLKey, defaultLeaseTTL).Duration(),
	}
}

// WithAddrs sets the client addresses.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDialTimeout sets the client dial timeout.
func WithDialTimeout(dialTimeout time.Duration) Option {
	return func(o *options) { o.dialTimeout = dialTimeout }
}

// WithClient sets the external client.
func WithClient(client *clientv3.Client) Option {
	return func(o *options) { o.client = client }
}

// WithNamespace sets the namespace.
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithTimeout sets the context timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithUsername sets the username.
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword sets the password.
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithLeaseTTL sets the lease TTL.
func WithLeaseTTL(leaseTTL time.Duration) Option {
	return func(o *options) { o.leaseTTL = leaseTTL }
}
