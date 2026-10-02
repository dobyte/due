package file

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultPath          = "./log/due.log"
	defaultMaxAge        = "7d"
	defaultMaxSize       = "500M"
	defaultBufferSize    = "32K"
	defaultRotate        = RotateNone
	defaultCompress      = false
	defaultFormat        = FormatText
	defaultFlushInterval = "1s"
)

const (
	defaultPathKey          = "etc.log.file.path"
	defaultFormatKey        = "etc.log.file.format"
	defaultMaxAgeKey        = "etc.log.file.maxAge"
	defaultMaxSizeKey       = "etc.log.file.maxSize"
	defaultBufferSizeKey    = "etc.log.file.bufferSize"
	defaultRotateKey        = "etc.log.file.rotate"
	defaultCompressKey      = "etc.log.file.compress"
	defaultFlushIntervalKey = "etc.log.file.flushInterval"
)

// Option configures the syncer.
type Option func(o *options)

type options struct {
	path          string        // File path
	format        Format        // Output format
	maxAge        time.Duration // Maximum retention time of a file
	maxSize       int64         // Maximum size of a single file
	bufferSize    int           // Buffer size
	rotate        Rotate        // File rotation rule
	compress      bool          // Whether to compress rotated log files
	flushInterval time.Duration // Flush interval; <=0 flushes every record immediately, >0 batches and flushes periodically
}

func defaultOptions() *options {
	return &options{
		path:          etc.Get(defaultPathKey, defaultPath).String(),
		format:        Format(etc.Get(defaultFormatKey, defaultFormat).String()),
		maxAge:        etc.Get(defaultMaxAgeKey, defaultMaxAge).Duration(),
		maxSize:       int64(etc.Get(defaultMaxSizeKey, defaultMaxSize).B()),
		bufferSize:    int(etc.Get(defaultBufferSizeKey, defaultBufferSize).B()),
		rotate:        Rotate(etc.Get(defaultRotateKey, defaultRotate).String()),
		compress:      etc.Get(defaultCompressKey, defaultCompress).Bool(),
		flushInterval: etc.Get(defaultFlushIntervalKey, defaultFlushInterval).Duration(),
	}
}

// WithPath sets the file path.
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithFormat sets the output format.
func WithFormat(format Format) Option {
	return func(o *options) { o.format = format }
}

// WithMaxAge sets the maximum retention time of a file.
func WithMaxAge(maxAge time.Duration) Option {
	return func(o *options) { o.maxAge = maxAge }
}

// WithMaxSize sets the maximum size of a single file.
func WithMaxSize(maxSize int64) Option {
	return func(o *options) { o.maxSize = maxSize }
}

// WithBufferSize sets the buffer size.
func WithBufferSize(bufferSize int) Option {
	return func(o *options) { o.bufferSize = bufferSize }
}

// WithRotate sets the file rotation rule.
func WithRotate(rotate Rotate) Option {
	return func(o *options) { o.rotate = rotate }
}

// WithCompress sets whether to compress rotated log files.
func WithCompress(compress bool) Option {
	return func(o *options) { o.compress = compress }
}

// WithFlushInterval sets the flush policy: <=0 flushes every log record immediately, >0 batches writes and flushes periodically.
func WithFlushInterval(flushInterval time.Duration) Option {
	return func(o *options) { o.flushInterval = flushInterval }
}
