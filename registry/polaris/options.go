package polaris

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/polarismesh/polaris-go/api"
)

const (
	defaultUrl       = "127.0.0.1:8091"
	defaultNamespace = "default"
	defaultTimeout   = "3s"
	defaultProtocol  = "grpc"
)

const (
	defaultUrlsKey      = "etc.registry.polaris.urls"
	defaultNamespaceKey = "etc.registry.polaris.namespace"
	defaultTimeoutKey   = "etc.registry.polaris.timeout"
	defaultProtocolKey  = "etc.registry.polaris.protocol"
)

type Option func(o *options)

// options holds the Polaris registry configuration.
type options struct {
	// Server address as ip:port.
	// Defaults to []string{127.0.0.1:8091}.
	urls []string

	// External SDK context.
	// When an external SDK context is provided it takes precedence over the built-in one;
	// defaults to nil.
	client api.SDKContext

	// Namespace.
	// Defaults to default.
	namespace string

	// Timeout for requests to the Polaris server.
	// Defaults to 3 seconds.
	timeout time.Duration

	// Protocol for communicating with the Polaris server.
	// Defaults to grpc.
	protocol string
}

func defaultOptions() *options {
	return &options{
		urls:      etc.Get(defaultUrlsKey, []string{defaultUrl}).Strings(),
		namespace: etc.Get(defaultNamespaceKey, defaultNamespace).String(),
		timeout:   etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		protocol:  etc.Get(defaultProtocolKey, defaultProtocol).String(),
	}
}

// WithUrls sets the server addresses.
func WithUrls(urls ...string) Option {
	return func(o *options) { o.urls = urls }
}

// WithClient sets the external SDK context.
func WithClient(client api.SDKContext) Option {
	return func(o *options) { o.client = client }
}

// WithNamespace sets the namespace.
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithTimeout sets the timeout for requests to the Polaris server.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithProtocol sets the protocol for communicating with the Polaris server.
func WithProtocol(protocol string) Option {
	return func(o *options) { o.protocol = protocol }
}
