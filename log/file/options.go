package file

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultPath     = "./log/due.log"
	defaultMaxAge   = "7d"
	defaultMaxSize  = "100M"
	defaultRotate   = RotateDay
	defaultCompress = false
	defaultFormat   = FormatText
)

const (
	defaultPathKey     = "etc.log.file.path"
	defaultMaxAgeKey   = "etc.log.file.maxAge"
	defaultMaxSizeKey  = "etc.log.file.maxSize"
	defaultRotateKey   = "etc.log.file.rotate"
	defaultCompressKey = "etc.log.file.compress"
	defaultFormatKey   = "etc.log.file.format"
)

type Option func(o *options)

type options struct {
	path     string        // 文件路径
	maxAge   time.Duration // 文件最大留存时间
	maxSize  int64         // 单个文件最大尺寸
	rotate   Rotate        // 文件反转规则
	compress bool          // 是否对轮换的日志文件进行压缩
	format   Format        // 输出格式
}

func defaultOptions() *options {
	return &options{
		path:     etc.Get(defaultPathKey, defaultPath).String(),
		maxAge:   etc.Get(defaultMaxAgeKey, defaultMaxAge).Duration(),
		maxSize:  int64(etc.Get(defaultMaxSizeKey, defaultMaxSize).B()),
		rotate:   Rotate(etc.Get(defaultRotateKey, defaultRotate).String()),
		compress: etc.Get(defaultCompressKey, defaultCompress).Bool(),
		format:   Format(etc.Get(defaultFormatKey, defaultFormat).String()),
	}
}

// WithPath 设置文件路径
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMaxAge 设置文件最大留存时间
func WithMaxAge(maxAge time.Duration) Option {
	return func(o *options) { o.maxAge = maxAge }
}

// WithMaxSize 设置单个文件最大尺寸
func WithMaxSize(maxSize int64) Option {
	return func(o *options) { o.maxSize = maxSize }
}

// WithRotate 设置文件反转规则
func WithRotate(rotate Rotate) Option {
	return func(o *options) { o.rotate = rotate }
}

// WithCompress 设置是否对轮换日志文件进行压缩
func WithCompress(compress bool) Option {
	return func(o *options) { o.compress = compress }
}

// WithFormat 设置输出格式
func WithFormat(format Format) Option {
	return func(o *options) { o.format = format }
}
