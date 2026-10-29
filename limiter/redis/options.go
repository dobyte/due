package redis

import (
	"strings"
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/limiter"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr          = "127.0.0.1:6379" // Default client connection address
	defaultDB            = 0                // Default database number
	defaultMaxRetries    = 3                // Default maximum number of retries
	defaultPrefix        = "due:limiter"    // Default key prefix
	defaultTimeout       = "200ms"          // Default timeout of one rate limiting decision
	defaultFailurePolicy = "failOpen"       // Default failure policy
)

const (
	defaultAddrsKey         = "etc.limiter.redis.addrs"         // Connection address configuration key
	defaultDBKey            = "etc.limiter.redis.db"            // Database number configuration key
	defaultMaxRetriesKey    = "etc.limiter.redis.maxRetries"    // Maximum retry count configuration key
	defaultPrefixKey        = "etc.limiter.redis.prefix"        // Key prefix configuration key
	defaultUsernameKey      = "etc.limiter.redis.username"      // Username configuration key
	defaultPasswordKey      = "etc.limiter.redis.password"      // Password configuration key
	defaultCertFileKey      = "etc.limiter.redis.certFile"      // Client certificate configuration key
	defaultKeyFileKey       = "etc.limiter.redis.keyFile"       // Client key configuration key
	defaultCaFileKey        = "etc.limiter.redis.caFile"        // CA certificate configuration key
	defaultTimeoutKey       = "etc.limiter.redis.timeout"       // Decision timeout configuration key
	defaultFailurePolicyKey = "etc.limiter.redis.failurePolicy" // Failure policy configuration key
)

// Option configures the redis limiter options.
type Option func(o *options)

// options holds the redis limiter options.
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
	// External client configuration; when an external client exists it takes precedence, defaults
	// to nil.
	client redis.UniversalClient

	// Prefix.
	// Key prefix, defaults to due:limiter.
	prefix string

	// Timeout of one rate limiting decision, defaults to 200ms.
	// A value of zero or less leaves the deadline entirely to the caller's context.
	timeout time.Duration

	// Failure policy used when the backend cannot make a decision, defaults to FailOpen.
	failurePolicy limiter.FailurePolicy
}

// defaultOptions creates the default redis limiter options.
func defaultOptions() *options {
	return &options{
		addrs:         etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		db:            etc.Get(defaultDBKey, defaultDB).Int(),
		maxRetries:    etc.Get(defaultMaxRetriesKey, defaultMaxRetries).Int(),
		prefix:        etc.Get(defaultPrefixKey, defaultPrefix).String(),
		username:      etc.Get(defaultUsernameKey).String(),
		password:      etc.Get(defaultPasswordKey).String(),
		certFile:      etc.Get(defaultCertFileKey).String(),
		keyFile:       etc.Get(defaultKeyFileKey).String(),
		caFile:        etc.Get(defaultCaFileKey).String(),
		timeout:       etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		failurePolicy: parseFailurePolicy(etc.Get(defaultFailurePolicyKey, defaultFailurePolicy).String()),
	}
}

// WithAddrs sets the client connection addresses.
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

// WithMaxRetries sets the maximum number of retries of the built-in client.
func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

// WithClient sets the external client. An external client takes precedence over the built-in one,
// and its lifecycle is managed by the caller.
func WithClient(client redis.UniversalClient) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix sets the key prefix. The final key is prefix + ":" + name + ":" + key; an empty
// prefix disables the prefix.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithTimeout sets the timeout of one rate limiting decision. Zero or less leaves the deadline
// entirely to the caller's context.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithFailurePolicy sets the failure policy used when the backend cannot make a decision.
func WithFailurePolicy(policy limiter.FailurePolicy) Option {
	return func(o *options) { o.failurePolicy = policy }
}

// parseFailurePolicy parses the configured failure policy. Unknown values fall back to FailOpen.
func parseFailurePolicy(value string) limiter.FailurePolicy {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "failclosed", "fail_closed", "closed":
		return limiter.FailClosed
	default:
		return limiter.FailOpen
	}
}
