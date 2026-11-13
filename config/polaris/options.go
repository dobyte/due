package polaris

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	"github.com/polarismesh/polaris-go/api"
)

const (
	defaultMode      = config.ReadOnly
	defaultUrl       = "127.0.0.1:8091"
	defaultNamespace = "default"
	defaultGroup     = "default"
	defaultTimeout   = "3s"
	defaultProtocol  = "grpc"
)

const (
	defaultModeKey      = "etc.config.polaris.mode"
	defaultUrlsKey      = "etc.config.polaris.urls"
	defaultNamespaceKey = "etc.config.polaris.namespace"
	defaultGroupKey     = "etc.config.polaris.group"
	defaultFilesKey     = "etc.config.polaris.files"
	defaultTimeoutKey   = "etc.config.polaris.timeout"
	defaultProtocolKey  = "etc.config.polaris.protocol"
)

// Option is a config option.
type Option func(o *options)

// options are the config options.
type options struct {
	// Context, defaults to context.Background.
	ctx context.Context

	// Read-write mode; supports read-only, write-only and read-write modes,
	// defaulting to read-only.
	mode config.Mode

	// Server addresses in the form ip:port, defaulting to []string{"127.0.0.1:8091"}.
	urls []string

	// External SDK context; when present it takes precedence, defaulting to nil.
	client api.SDKContext

	// Namespace, defaulting to default.
	namespace string

	// Config group, defaulting to default.
	group string

	// List of file names to load, store and watch, defaulting to empty. When empty,
	// every file under the group is loaded and watched. An entry without "*" is an
	// exact file name; an entry containing "*" is a pattern where "*" matches any
	// sequence of characters. An explicit load or store of a file outside this list
	// is rejected.
	files []string

	// Timeout for requests to the Polaris server, defaulting to 3 seconds.
	timeout time.Duration

	// Protocol for communicating with the Polaris server, defaulting to grpc.
	protocol string
}

// defaultOptions returns the default config options.
func defaultOptions() *options {
	return &options{
		ctx:       context.Background(),
		mode:      config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
		urls:      etc.Get(defaultUrlsKey, []string{defaultUrl}).Strings(),
		namespace: etc.Get(defaultNamespaceKey, defaultNamespace).String(),
		group:     etc.Get(defaultGroupKey, defaultGroup).String(),
		files:     etc.Get(defaultFilesKey, []string{}).Strings(),
		timeout:   etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		protocol:  etc.Get(defaultProtocolKey, defaultProtocol).String(),
	}
}

// WithContext sets the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithMode sets the read-write mode.
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}

// WithUrls sets the server addresses.
func WithUrls(urls ...string) Option {
	return func(o *options) { o.urls = urls }
}

// WithClient sets the external SDK context. When one is supplied, the config
// source skips the client build process, and the caller is responsible for
// destroying the SDK context.
func WithClient(client api.SDKContext) Option {
	return func(o *options) { o.client = client }
}

// WithNamespace sets the namespace.
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithGroup sets the config group.
func WithGroup(group string) Option {
	return func(o *options) { o.group = group }
}

// WithFiles sets the list of file names to load, store and watch. When it is empty,
// every file under the group is available.
//
// An entry is an exact file name unless it contains "*", in which case it is a
// pattern where "*" matches any sequence of characters and every other character is
// literal; only "*" is supported as a wildcard. When the list is not empty, an
// explicit load or store of a file outside the list is rejected.
func WithFiles(files ...string) Option {
	return func(o *options) { o.files = files }
}

// WithTimeout sets the timeout for requests to the Polaris server.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithProtocol sets the protocol for communicating with the Polaris server.
func WithProtocol(protocol string) Option {
	return func(o *options) { o.protocol = protocol }
}
