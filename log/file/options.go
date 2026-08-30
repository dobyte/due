package file

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultPath       = "./log/due.log"
	defaultMaxAge     = "7d"
	defaultMaxSize    = "500M"
	defaultBufferSize = "32K"
	defaultRotate       = RotateNone
	defaultCompress     = false
	defaultFormat       = FormatText
	defaultFlushInterval = "1s"
)

const (
	defaultPathKey       = "etc.log.file.path"
	defaultFormatKey     = "etc.log.file.format"
	defaultMaxAgeKey     = "etc.log.file.maxAge"
	defaultMaxSizeKey    = "etc.log.file.maxSize"
	defaultBufferSizeKey = "etc.log.file.bufferSize"
	defaultRotateKey     = "etc.log.file.rotate"
	defaultCompressKey   = "etc.log.file.compress"
	defaultFlushIntervalKey = "etc.log.file.flushInterval"
)

type Option func(o *options)

type options struct {
	path       string        // 文件路径
	format     Format        // 输出格式
	maxAge     time.Duration // 文件最大留存时间
	maxSize    int64         // 单个文件最大尺寸
	bufferSize int           // 缓冲区大小
	rotate        Rotate        // 文件反转规则
	compress      bool          // 是否对轮换的日志文件进行压缩
	flushInterval time.Duration // 定时刷盘间隔
}

func defaultOptions() *options {
	return &options{
		path:       etc.Get(defaultPathKey, defaultPath).String(),
		format:     Format(etc.Get(defaultFormatKey, defaultFormat).String()),
		maxAge:     etc.Get(defaultMaxAgeKey, defaultMaxAge).Duration(),
		maxSize:    int64(etc.Get(defaultMaxSizeKey, defaultMaxSize).B()),
		bufferSize: int(etc.Get(defaultBufferSizeKey, defaultBufferSize).B()),
		rotate:     Rotate(etc.Get(defaultRotateKey, defaultRotate).String()),
		compress:   etc.Get(defaultCompressKey, defaultCompress).Bool(),
		flushInterval: etc.Get(defaultFlushIntervalKey, defaultFlushInterval).Duration(),
	}
}

// WithPath 设置文件路径
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithFormat 设置输出格式
func WithFormat(format Format) Option {
	return func(o *options) { o.format = format }
}

// WithMaxAge 设置文件最大留存时间
func WithMaxAge(maxAge time.Duration) Option {
	return func(o *options) { o.maxAge = maxAge }
}

// WithMaxSize 设置单个文件最大尺寸
func WithMaxSize(maxSize int64) Option {
	return func(o *options) { o.maxSize = maxSize }
}

// WithBufferSize 设置缓冲区大小
func WithBufferSize(bufferSize int) Option {
	return func(o *options) { o.bufferSize = bufferSize }
}

// WithRotate 设置文件反转规则
func WithRotate(rotate Rotate) Option {
	return func(o *options) { o.rotate = rotate }
}

// WithCompress 设置是否对轮换日志文件进行压缩
func WithCompress(compress bool) Option {
	return func(o *options) { o.compress = compress }
}

// WithFlushInterval 设置定时刷盘间隔，<=0 表示禁用定时刷盘
func WithFlushInterval(flushInterval time.Duration) Option {
	return func(o *options) { o.flushInterval = flushInterval }
}
