package redis

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr          = "127.0.0.1:6379"
	defaultDB            = 0
	defaultMaxRetries    = 3
	defaultPrefix        = "due:cache"
	defaultNilValue      = "cache@nil"
	defaultNilExpiration = "10s"
	defaultMinExpiration = "1h"
	defaultMaxExpiration = "24h"
)

const (
	defaultAddrsKey         = "etc.cache.redis.addrs"
	defaultDBKey            = "etc.cache.redis.db"
	defaultMaxRetriesKey    = "etc.cache.redis.maxRetries"
	defaultPrefixKey        = "etc.cache.redis.prefix"
	defaultUsernameKey      = "etc.cache.redis.username"
	defaultPasswordKey      = "etc.cache.redis.password"
	defaultCertFileKey      = "etc.cache.redis.certFile"
	defaultKeyFileKey       = "etc.cache.redis.keyFile"
	defaultCAFileKey        = "etc.cache.redis.caFile"
	defaultNilValueKey      = "etc.cache.redis.nilValue"
	defaultNilExpirationKey = "etc.cache.redis.nilExpiration"
	defaultMinExpirationKey = "etc.cache.redis.minExpiration"
	defaultMaxExpirationKey = "etc.cache.redis.maxExpiration"
)

type Option func(o *options)

type options struct {
	// Client connection addresses.
	// Built-in client configuration; defaults to []string{"127.0.0.1:6379"}.
	addrs []string

	// Database number.
	// Built-in client configuration; defaults to 0.
	db int

	// Username.
	// Built-in client configuration; defaults to empty.
	username string

	// Password.
	// Built-in client configuration; defaults to empty.
	password string

	// Client certificate file.
	certFile string

	// Client private key file.
	keyFile string

	// CA certificate file.
	caFile string

	// Maximum number of retries.
	// Built-in client configuration; defaults to 3.
	maxRetries int

	// Client.
	// External client configuration. When an external client is set, it takes precedence over the
	// built-in one; defaults to nil.
	client redis.UniversalClient

	// Prefix.
	// Key prefix; defaults to due:cache.
	prefix string

	// Nil value; defaults to cache@nil.
	nilValue string

	// Nil value expiration time; defaults to 10s.
	nilExpiration time.Duration

	// Minimum expiration time; defaults to 1h.
	minExpiration time.Duration

	// Maximum expiration time; defaults to 24h.
	maxExpiration time.Duration
}

func defaultOptions() *options {
	return &options{
		addrs:         etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		db:            etc.Get(defaultDBKey, defaultDB).Int(),
		username:      etc.Get(defaultUsernameKey).String(),
		password:      etc.Get(defaultPasswordKey).String(),
		certFile:      etc.Get(defaultCertFileKey).String(),
		keyFile:       etc.Get(defaultKeyFileKey).String(),
		caFile:        etc.Get(defaultCAFileKey).String(),
		maxRetries:    etc.Get(defaultMaxRetriesKey, defaultMaxRetries).Int(),
		prefix:        etc.Get(defaultPrefixKey, defaultPrefix).String(),
		nilValue:      etc.Get(defaultNilValueKey, defaultNilValue).String(),
		nilExpiration: etc.Get(defaultNilExpirationKey, defaultNilExpiration).Duration(),
		minExpiration: etc.Get(defaultMinExpirationKey, defaultMinExpiration).Duration(),
		maxExpiration: etc.Get(defaultMaxExpirationKey, defaultMaxExpiration).Duration(),
	}
}

// WithAddrs sets the connection addresses. It accepts one or more Redis node addresses.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDB sets the database number.
func WithDB(db int) Option {
	return func(o *options) { o.db = db }
}

// WithUsername sets the username used for authentication.
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword sets the password used for authentication.
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithCredentials sets the client certificate, the client private key and the CA certificate. Each
// argument is a file path.
func WithCredentials(certFile, keyFile, caFile string) Option {
	return func(o *options) { o.certFile, o.keyFile, o.caFile = certFile, keyFile, caFile }
}

// WithMaxRetries sets the maximum number of retries.
func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

// WithClient sets an external client. The external client takes precedence over the built-in one.
func WithClient(client redis.UniversalClient) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix sets the key prefix.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithNilValue sets the placeholder value that marks a cached nil.
func WithNilValue(nilValue string) Option {
	return func(o *options) { o.nilValue = nilValue }
}

// WithNilExpiration sets the expiration time of a cached nil. Non-positive values are ignored.
func WithNilExpiration(nilExpiration time.Duration) Option {
	return func(o *options) {
		if nilExpiration > 0 {
			o.nilExpiration = nilExpiration
		}
	}
}

// WithMinExpiration sets the minimum expiration time. Non-positive values are ignored.
func WithMinExpiration(minExpiration time.Duration) Option {
	return func(o *options) {
		if minExpiration > 0 {
			o.minExpiration = minExpiration
		}
	}
}

// WithMaxExpiration sets the maximum expiration time. Non-positive values are ignored.
func WithMaxExpiration(maxExpiration time.Duration) Option {
	return func(o *options) {
		if maxExpiration > 0 {
			o.maxExpiration = maxExpiration
		}
	}
}
