package log

import (
	"reflect"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log/console"
	"github.com/dobyte/due/v2/log/file"
)

const (
	defaultLevel          = LevelInfo
	defaultStackLevel     = LevelError
	defaultCallerDepth    = 2
	defaultCallerFullPath = false
	defaultTimeFormat     = "2006/01/02 15:04:05.000000"
)

const (
	defaultLevelKey          = "etc.log.level"
	defaultTerminalsKey      = "etc.log.terminals"
	defaultStackLevelKey     = "etc.log.stackLevel"
	defaultCallerDepthKey    = "etc.log.callerDepth"
	defaultCallerFullPathKey = "etc.log.callerFullPath"
	defaultTimeFormatKey     = "etc.log.timeFormat"
)

var (
	defaultSyncers   = []Syncer{console.NewSyncer(), file.NewSyncer()}
	defaultTerminals = []Terminal{TerminalFile}
)

type Option func(o *options)

type options struct {
	level          Level    // 输出级别
	terminals      any      // 输出终端
	stackLevel     Level    // 输出栈的日志级别
	callerDepth    int      // 输出栈的深度
	callerFullPath bool     // 输出栈的调用文件全路径
	timeFormat     string   // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
	syncers        []Syncer // 日志同步器
}

func defaultOptions() *options {
	opts := &options{
		level:          Level(etc.Get(defaultLevelKey, defaultLevel).String()),
		syncers:        defaultSyncers,
		terminals:      defaultTerminals,
		stackLevel:     Level(etc.Get(defaultStackLevelKey, defaultStackLevel).String()),
		callerDepth:    etc.Get(defaultCallerDepthKey, defaultCallerDepth).Int(),
		callerFullPath: etc.Get(defaultCallerFullPathKey, defaultCallerFullPath).Bool(),
		timeFormat:     etc.Get(defaultTimeFormatKey, defaultTimeFormat).String(),
	}

	switch value := etc.Get(defaultTerminalsKey); value.Kind() {
	case reflect.Slice, reflect.Array:
		terminals := make([]string, 0)

		if err := value.Scan(&terminals); err != nil || len(terminals) == 0 {
			opts.terminals = defaultTerminals
		} else {
			opts.terminals = terminals
		}
	case reflect.Map:
		terminals := make(map[string][]Level)

		if err := value.Scan(&terminals); err != nil || len(terminals) == 0 {
			opts.terminals = defaultTerminals
		} else {
			opts.terminals = terminals
		}
	default:
		opts.terminals = defaultTerminals
	}

	return opts
}

// WithLevel 设置日志的输出级别
func WithLevel(level Level) Option {
	return func(o *options) { o.level = level }
}

// WithSyncers 设置日志同步器
func WithSyncers(syncers ...Syncer) Option {
	return func(o *options) {
		for _, syncer := range o.syncers {
			_ = syncer.Close()
		}

		o.syncers = syncers
	}
}

// WithTerminals 设置日志的输出终端
func WithTerminals[T []Terminal | map[Terminal][]Level](terminals T) Option {
	return func(o *options) { o.terminals = terminals }
}

// WithStackLevel 设置日志的输出栈的日志级别
func WithStackLevel(level Level) Option {
	return func(o *options) { o.stackLevel = level }
}

// WithStackDepth 设置日志的输出栈的深度
func WithCallerDepth(depth int) Option {
	return func(o *options) { o.callerDepth = depth }
}

// WithCallerFullPath 设置日志的输出栈的调用文件全路径
func WithCallerFullPath(fullPath bool) Option {
	return func(o *options) { o.callerFullPath = fullPath }
}

// WithTimeFormat 设置日志输出时间格式
func WithTimeFormat(timeFormat string) Option {
	return func(o *options) { o.timeFormat = timeFormat }
}
