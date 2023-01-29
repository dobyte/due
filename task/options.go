package task

import "github.com/dobyte/due/config"

const (
	defaultSize         = 100000 // 默认任务池大小
	defaultNonblocking  = true   // 默认是否非阻塞
	defaultDisablePurge = true   // 默认是否禁用清除
)

const (
	defaultSizeKey         = "config.taskPool.size"         // 任务池大小
	defaultNonblockingKey  = "config.taskPool.nonblocking"  // 是否非阻塞
	defaultDisablePurgeKey = "config.taskPool.disablePurge" // 是否禁用清除
)

type options struct {
	size         int  // 任务池大小
	nonblocking  bool // 是否非阻塞
	disablePurge bool // 是否禁用清除
}

type Option func(o *options)

func defaultOptions() *options {
	opts := &options{
		size:         defaultSize,
		nonblocking:  defaultNonblocking,
		disablePurge: defaultDisablePurge,
	}

	if size := config.Get(defaultSizeKey).Int(); size > 0 {
		opts.size = size
	}

	opts.nonblocking = config.Get(defaultNonblockingKey, defaultNonblocking).Bool()
	opts.disablePurge = config.Get(defaultDisablePurgeKey, defaultDisablePurge).Bool()

	return opts
}

// WithSize 设置任务池大小
func WithSize(size int) Option {
	return func(o *options) { o.size = size }
}

// WithNonblocking 设置是否非阻塞
func WithNonblocking(nonblocking bool) Option {
	return func(o *options) { o.nonblocking = nonblocking }
}

// WithDisablePurge 设置是否禁用清除
func WithDisablePurge(disablePurge bool) Option {
	return func(o *options) { o.disablePurge = disablePurge }
}
