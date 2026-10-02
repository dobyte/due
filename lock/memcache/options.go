package memcache

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultAddr              = "127.0.0.1:11211" // Default client connection address
	defaultPrefix            = "due:lock"        // Default key prefix
	defaultExpiration        = "3s"              // Default lock expiration
	defaultAcquireInterval   = "20ms"            // Default interval between lock acquisition attempts
	defaultAcquireMaxRetries = 0                 // Default maximum number of lock acquisition retries; 0 means unlimited retries
)

const (
	defaultAddrsKey             = "etc.lock.memcache.addrs"             // Connection addresses config key
	defaultPrefixKey            = "etc.lock.memcache.prefix"            // Key prefix config key
	defaultExpirationKey        = "etc.lock.memcache.expiration"        // Lock expiration config key
	defaultAcquireIntervalKey   = "etc.lock.memcache.acquireInterval"   // Lock acquisition interval config key
	defaultAcquireMaxRetriesKey = "etc.lock.memcache.acquireMaxRetries" // Lock acquisition max retries config key
)

// Option is a lock configuration function.
type Option func(o *options)

// options holds the lock configuration.
//
// It controls the memcached client, the key prefix, the lock expiration and the lock acquisition
// behavior. Every field is read from the configuration environment first and falls back to a
// default when it is not configured.
type options struct {
	// Client connection addresses.
	// Configured for the built-in client; defaults to []string{"127.0.0.1:11211"}.
	addrs []string

	// Client.
	// An external client; when set, it is preferred over the built-in client. Defaults to nil.
	client *memcache.Client

	// Prefix.
	// The key prefix; defaults to due:lock.
	prefix string

	// Lock expiration; defaults to 3s.
	// Note: memcached expiration has second granularity and 0 means never expires, so NewMaker
	// clamps an expiration below 1s to 1s. memcached records the expiration in whole seconds, so the
	// actual lifetime may be nearly one second shorter than the configured value. Acquire and
	// TryAcquire without a fixed expiration renew automatically at half of the expiration; when the
	// expiration is 2s or less the renewal can hardly finish before the lock expires, so a value of
	// at least 3s is recommended.
	expiration time.Duration

	// Interval between lock acquisition attempts; defaults to 20ms.
	// Note: a value of 0 or less is clamped to the default 20ms in NewMaker to avoid the retry loop
	// degenerating into a busy-wait without backoff.
	acquireInterval time.Duration

	// Maximum number of lock acquisition retries; defaults to unlimited.
	acquireMaxRetries int
}

// defaultOptions creates the default lock configuration.
//
// It reads every parameter from the configuration environment in turn and fills in the built-in
// defaults for the ones that are not configured.
func defaultOptions() *options {
	return &options{
		addrs:             etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		prefix:            etc.Get(defaultPrefixKey, defaultPrefix).String(),
		expiration:        etc.Get(defaultExpirationKey, defaultExpiration).Duration(),
		acquireInterval:   etc.Get(defaultAcquireIntervalKey, defaultAcquireInterval).Duration(),
		acquireMaxRetries: etc.Get(defaultAcquireMaxRetriesKey, defaultAcquireMaxRetries).Int(),
	}
}

// WithAddrs sets the client connection addresses.
//
// It sets the connection addresses of the built-in client and takes effect only when no external
// client is specified.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithClient sets an external client.
//
// It sets an external client, which is preferred when present; in that case the Maker no longer
// manages the client's lifecycle when it is closed.
func WithClient(client *memcache.Client) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix sets the key prefix.
//
// The final lock key is prefix + ":" + name; when the prefix is empty no concatenation happens.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithExpiration sets the lock expiration.
//
// It sets the expiration of the lock, which is also the target duration refreshed by the background
// renewal. memcached has second granularity, so a value below 1s is clamped to 1s. Note: memcached
// records the expiration in whole seconds, so the actual lifetime may be nearly one second shorter
// than the configured value. The background renewal interval is half of the expiration, so an
// expiration below 2s may make it hard for the renewal to finish before the lock expires; for a
// renewal scenario that needs to hold the lock continuously, a value of at least 3s is recommended.
func WithExpiration(expiration time.Duration) Option {
	return func(o *options) { o.expiration = expiration }
}

// WithAcquireInterval sets the interval between lock acquisition attempts.
//
// It is the interval between retries after a failed lock acquisition; a value of 0 or less is
// clamped to the default 20ms in NewMaker.
func WithAcquireInterval(acquireInterval time.Duration) Option {
	return func(o *options) { o.acquireInterval = acquireInterval }
}

// WithAcquireMaxRetries sets the maximum number of lock acquisition retries.
//
// When the maximum number of retries is reached without acquiring the lock, a timeout error is
// returned; 0 means unlimited retries until success or context cancellation.
func WithAcquireMaxRetries(acquireMaxRetries int) Option {
	return func(o *options) { o.acquireMaxRetries = acquireMaxRetries }
}
