package task

import (
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultSize         = 100000 // Default task pool size
	defaultNonblocking  = true   // Whether non-blocking is the default
	defaultDisablePurge = true   // Whether purging is disabled by default
)

const (
	defaultSizeKey         = "etc.task.size"         // Task pool size
	defaultNonblockingKey  = "etc.task.nonblocking"  // Whether non-blocking
	defaultDisablePurgeKey = "etc.task.disablePurge" // Whether purging is disabled
)

type options struct {
	size         int  // Task pool size
	nonblocking  bool // Whether non-blocking
	disablePurge bool // Whether purging is disabled
}

// Option configures the task pool.
type Option func(o *options)

// defaultOptions returns the default options, preferring values read from the configuration
// center.
func defaultOptions() *options {
	opts := &options{
		size:         defaultSize,
		nonblocking:  defaultNonblocking,
		disablePurge: defaultDisablePurge,
	}

	if size := etc.Get(defaultSizeKey).Int(); size > 0 {
		opts.size = size
	}

	opts.nonblocking = etc.Get(defaultNonblockingKey, defaultNonblocking).Bool()
	opts.disablePurge = etc.Get(defaultDisablePurgeKey, defaultDisablePurge).Bool()

	return opts
}

// WithSize returns an Option that sets the task pool size.
func WithSize(size int) Option {
	return func(o *options) { o.size = size }
}

// WithNonblocking returns an Option that sets whether the pool is non-blocking.
//
// In non-blocking mode, a task submission returns an error immediately when the pool is full
// instead of blocking and waiting.
func WithNonblocking(nonblocking bool) Option {
	return func(o *options) { o.nonblocking = nonblocking }
}

// WithDisablePurge returns an Option that sets whether purging is disabled.
//
// When purging is disabled, idle goroutines are not reclaimed periodically.
func WithDisablePurge(disablePurge bool) Option {
	return func(o *options) { o.disablePurge = disablePurge }
}
