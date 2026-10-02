package redis

import (
	"context"

	"github.com/dobyte/due/v2/etc"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr       = "127.0.0.1:6379" // Default connection address
	defaultDB         = 0                // Default database number
	defaultMaxRetries = 3                // Default maximum number of retries
	defaultRetryTimes = 3                // Default reconnect retry count
	defaultPrefix     = "due:locate"     // Default key prefix
)

const (
	defaultAddrsKey      = "etc.locate.redis.addrs"
	defaultDBKey         = "etc.locate.redis.db"
	defaultUsernameKey   = "etc.locate.redis.username"
	defaultPasswordKey   = "etc.locate.redis.password"
	defaultCertFileKey   = "etc.locate.redis.certFile"
	defaultKeyFileKey    = "etc.locate.redis.keyFile"
	defaultCAFileKey     = "etc.locate.redis.caFile"
	defaultMaxRetriesKey = "etc.locate.redis.maxRetries"
	defaultPrefixKey     = "etc.locate.redis.prefix"
)

// Option configures the locator options.
type Option func(o *options)

// options holds the locator options.
type options struct {
	ctx context.Context // Context

	// Connection addresses of the client.
	// Built-in client configuration, defaults to []string{"127.0.0.1:6379"}.
	addrs []string

	// Database number.
	// Built-in client configuration, defaults to 0.
	db int

	// Username.
	// Built-in client configuration, defaults to empty.
	username string

	// Password.
	// Built-in client configuration, defaults to empty.
	password string

	// Client certificate.
	certFile string

	// Client key.
	keyFile string

	// CA certificate.
	caFile string

	// Maximum number of retries.
	// Built-in client configuration, defaults to 3.
	maxRetries int

	// Client.
	// External client configuration; when an external client exists it takes precedence, defaults to nil.
	client redis.UniversalClient

	// Prefix.
	// Key prefix, defaults to due:locate.
	prefix string
}

// defaultOptions creates the default locator options.
//
// It reads every parameter from the configuration environment and fills in defaults.
func defaultOptions() *options {
	return &options{
		ctx:        context.Background(),
		addrs:      etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		db:         etc.Get(defaultDBKey, defaultDB).Int(),
		username:   etc.Get(defaultUsernameKey).String(),
		password:   etc.Get(defaultPasswordKey).String(),
		certFile:   etc.Get(defaultCertFileKey).String(),
		keyFile:    etc.Get(defaultKeyFileKey).String(),
		caFile:     etc.Get(defaultCAFileKey).String(),
		maxRetries: etc.Get(defaultMaxRetriesKey, defaultMaxRetries).Int(),
		prefix:     etc.Get(defaultPrefixKey, defaultPrefix).String(),
	}
}

// WithContext sets the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithAddrs sets the connection addresses.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDB sets the database number.
func WithDB(db int) Option {
	return func(o *options) { o.db = db }
}

// WithUsername sets the username.
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword sets the password.
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithCredentials sets the certificate, key and CA certificate.
func WithCredentials(certFile, keyFile, caFile string) Option {
	return func(o *options) { o.certFile, o.keyFile, o.caFile = certFile, keyFile, caFile }
}

// WithMaxRetries sets the maximum number of retries.
func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

// WithClient sets the external client.
func WithClient(client redis.UniversalClient) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix sets the prefix.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}
