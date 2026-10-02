package redis

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr          = "127.0.0.1:6379"
	defaultDB            = 0
	defaultMaxRetries    = 3
	defaultPrefix        = "due:eventbus"
	defaultStaleDuration = 0
)

const (
	defaultAddrsKey         = "etc.eventbus.redis.addrs"
	defaultDBKey            = "etc.eventbus.redis.db"
	defaultUsernameKey      = "etc.eventbus.redis.username"
	defaultPasswordKey      = "etc.eventbus.redis.password"
	defaultCertFileKey      = "etc.eventbus.redis.certFile"
	defaultKeyFileKey       = "etc.eventbus.redis.keyFile"
	defaultCAFileKey        = "etc.eventbus.redis.caFile"
	defaultMaxRetriesKey    = "etc.eventbus.redis.maxRetries"
	defaultPrefixKey        = "etc.eventbus.redis.prefix"
	defaultStaleDurationKey = "etc.eventbus.redis.staleDuration"
)

type Option func(o *options)

type options struct {
	ctx context.Context

	// Client connection addresses.
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
	// External client configuration; when set, the external client is preferred. Defaults to nil.
	client redis.UniversalClient

	// Prefix.
	// Key prefix, defaults to due:eventbus.
	prefix string

	// Message retention duration.
	// Unconsumed messages older than this are dropped automatically. Defaults to 0, which keeps no
	// messages and drops unconsumed messages immediately.
	staleDuration time.Duration
}

func defaultOptions() *options {
	return &options{
		ctx:           context.Background(),
		addrs:         etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		db:            etc.Get(defaultDBKey, defaultDB).Int(),
		username:      etc.Get(defaultUsernameKey).String(),
		password:      etc.Get(defaultPasswordKey).String(),
		certFile:      etc.Get(defaultCertFileKey).String(),
		keyFile:       etc.Get(defaultKeyFileKey).String(),
		caFile:        etc.Get(defaultCAFileKey).String(),
		maxRetries:    etc.Get(defaultMaxRetriesKey, defaultMaxRetries).Int(),
		prefix:        etc.Get(defaultPrefixKey, defaultPrefix).String(),
		staleDuration: etc.Get(defaultStaleDurationKey, defaultStaleDuration).Duration(),
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

// WithStaleDuration sets the message retention duration.
func WithStaleDuration(staleDuration time.Duration) Option {
	return func(o *options) { o.staleDuration = staleDuration }
}
