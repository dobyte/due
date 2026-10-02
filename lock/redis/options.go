package redis

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr              = "127.0.0.1:6379" // Default client connection address
	defaultDB                = 0                // Default database number
	defaultMaxRetries        = 3                // Default maximum number of retries
	defaultPrefix            = "due:lock"       // Default key prefix
	defaultExpiration        = "3s"             // Default lock expiration
	defaultAcquireInterval   = "20ms"           // Default interval of the acquire loop
	defaultAcquireMaxRetries = 0                // Default maximum number of retries of the acquire loop; 0 means unlimited retries
)

const (
	defaultAddrsKey             = "etc.lock.redis.addrs"             // Connection address configuration key
	defaultDBKey                = "etc.lock.redis.db"                // Database number configuration key
	defaultMaxRetriesKey        = "etc.lock.redis.maxRetries"        // Maximum retry count configuration key
	defaultPrefixKey            = "etc.lock.redis.prefix"            // Key prefix configuration key
	defaultUsernameKey          = "etc.lock.redis.username"          // Username configuration key
	defaultPasswordKey          = "etc.lock.redis.password"          // Password configuration key
	defaultCertFileKey          = "etc.lock.redis.certFile"          // Client certificate configuration key
	defaultKeyFileKey           = "etc.lock.redis.keyFile"           // Client key configuration key
	defaultCaFileKey            = "etc.lock.redis.caFile"            // CA certificate configuration key
	defaultExpirationKey        = "etc.lock.redis.expiration"        // Lock expiration configuration key
	defaultAcquireIntervalKey   = "etc.lock.redis.acquireInterval"   // Acquire loop interval configuration key
	defaultAcquireMaxRetriesKey = "etc.lock.redis.acquireMaxRetries" // Acquire loop maximum retry count configuration key
)

// Option configures the lock options.
type Option func(o *options)

// options holds the lock options.
//
// It controls the redis client, the key prefix, the lock expiration and the behavior of the acquire
// loop. Every parameter is read from the configuration environment first and falls back to a
// default when it is not configured.
type options struct {
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
	// Built-in client TLS configuration, defaults to empty.
	certFile string

	// Client key.
	// Built-in client TLS configuration, defaults to empty.
	keyFile string

	// CA certificate.
	// Built-in client TLS configuration, defaults to empty.
	caFile string

	// Maximum number of retries.
	// Built-in client configuration, defaults to 3.
	maxRetries int

	// Client.
	// External client configuration; when an external client exists it takes precedence, defaults to nil.
	client redis.UniversalClient

	// Prefix.
	// Key prefix, defaults to due:lock.
	prefix string

	// Lock expiration, defaults to 3s.
	// Note that the redis expiration precision is milliseconds; NewMaker clamps a value shorter than
	// 1 millisecond to 1 millisecond.
	expiration time.Duration

	// Interval of the acquire loop, defaults to 20ms.
	acquireInterval time.Duration

	// Maximum number of retries of the acquire loop, defaults to unlimited.
	acquireMaxRetries int
}

// defaultOptions creates the default lock options.
//
// It reads every parameter from the configuration environment in turn and fills in defaults, using
// the built-in defaults when a value is not configured.
func defaultOptions() *options {
	return &options{
		addrs:             etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		db:                etc.Get(defaultDBKey, defaultDB).Int(),
		maxRetries:        etc.Get(defaultMaxRetriesKey, defaultMaxRetries).Int(),
		prefix:            etc.Get(defaultPrefixKey, defaultPrefix).String(),
		username:          etc.Get(defaultUsernameKey).String(),
		password:          etc.Get(defaultPasswordKey).String(),
		certFile:          etc.Get(defaultCertFileKey).String(),
		keyFile:           etc.Get(defaultKeyFileKey).String(),
		caFile:            etc.Get(defaultCaFileKey).String(),
		expiration:        etc.Get(defaultExpirationKey, defaultExpiration).Duration(),
		acquireInterval:   etc.Get(defaultAcquireIntervalKey, defaultAcquireInterval).Duration(),
		acquireMaxRetries: etc.Get(defaultAcquireMaxRetriesKey, defaultAcquireMaxRetries).Int(),
	}
}

// WithAddrs sets the client connection addresses.
//
// It sets the connection addresses of the built-in client and takes effect when no external client
// is specified.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDB sets the database number.
//
// It sets the database number used by the built-in client and takes effect when no external client
// is specified.
func WithDB(db int) Option {
	return func(o *options) { o.db = db }
}

// WithUsername sets the username.
//
// It sets the authentication username of the built-in client and takes effect when no external
// client is specified.
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword sets the password.
//
// It sets the authentication password of the built-in client and takes effect when no external
// client is specified.
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithCredentials sets the certificate, key and CA certificate.
//
// It sets the mutual TLS configuration of the built-in client and takes effect when no external
// client is specified; NewMaker builds the TLS configuration only when all three parameters are
// non-empty.
func WithCredentials(certFile, keyFile, caFile string) Option {
	return func(o *options) { o.certFile, o.keyFile, o.caFile = certFile, keyFile, caFile }
}

// WithMaxRetries sets the maximum number of retries.
//
// It sets the maximum number of retries of the built-in client when a network request fails and
// takes effect when no external client is specified.
func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

// WithClient sets the external client.
//
// It sets an external client, which takes precedence when it exists; in that case closing the maker
// no longer manages the lifecycle of the client.
func WithClient(client redis.UniversalClient) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix sets the prefix.
//
// It sets the key prefix; the final lock key is prefix + ":" + name, and no prefix is joined when it
// is empty.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithExpiration sets the lock expiration.
//
// It is the lock expiration and also the target duration refreshed by the background renewal. The
// redis precision is milliseconds, so a value shorter than 1 millisecond is clamped to 1
// millisecond. Note that the background renewal interval is half of the expiration, so a very short
// expiration (on the order of tens of milliseconds) makes it hard for renewal to refresh the lock
// before it expires; for renewal scenarios that need to hold the lock continuously, a value of at
// least 1s is recommended.
func WithExpiration(expiration time.Duration) Option {
	return func(o *options) { o.expiration = expiration }
}

// WithAcquireInterval sets the interval between acquire attempts.
//
// It is the interval before retrying after an acquire attempt in the loop fails; when it is less
// than or equal to 0, NewMaker clamps it to the default of 20ms.
func WithAcquireInterval(acquireInterval time.Duration) Option {
	return func(o *options) { o.acquireInterval = acquireInterval }
}

// WithAcquireMaxRetries sets the maximum number of retries of the acquire loop.
//
// When the maximum number of retries is reached without a successful acquisition, a timeout error
// is returned; 0 means unlimited retries until success or context cancellation.
func WithAcquireMaxRetries(acquireMaxRetries int) Option {
	return func(o *options) { o.acquireMaxRetries = acquireMaxRetries }
}
