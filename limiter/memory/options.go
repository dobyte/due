package memory

import "time"

const (
	defaultPrefix          = "due:limiter" // Default key prefix
	defaultCleanupInterval = time.Minute   // Default interval of the expired bucket cleanup
)

// Option configures the memory limiter maker.
type Option func(o *options)

// options holds the memory limiter configuration.
type options struct {
	// Prefix.
	// Key prefix; defaults to due:limiter. An explicitly empty prefix disables it.
	prefix string

	// Cleanup interval of expired buckets; defaults to one minute.
	// A value of zero or less is clamped to the default in NewMaker.
	cleanupInterval time.Duration
}

// defaultOptions creates the default memory limiter configuration.
func defaultOptions() *options {
	return &options{
		prefix:          defaultPrefix,
		cleanupInterval: defaultCleanupInterval,
	}
}

// WithPrefix sets the key prefix. The final key is prefix + ":" + name + ":" + key; an empty
// prefix disables the prefix.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithCleanupInterval sets the interval of the expired bucket cleanup. A value of zero or less is
// clamped to the default of one minute.
func WithCleanupInterval(interval time.Duration) Option {
	return func(o *options) { o.cleanupInterval = interval }
}
