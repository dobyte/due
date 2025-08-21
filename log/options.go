package log

import (
	"reflect"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultLevel        = LevelInfo
	defaultStackLevel   = LevelError
	defaultCallDepth    = 2
	defaultCallFullPath = false
	defaultTimeFormat   = "2006/01/02 15:04:05.000000"
)

const (
	defaultLevelKey        = "etc.log.level"
	defaultTerminalsKey    = "etc.log.terminals"
	defaultStackLevelKey   = "etc.log.stackLevel"
	defaultCallDepthKey    = "etc.log.callDepth"
	defaultCallFullPathKey = "etc.log.callFullPath"
	defaultTimeFormatKey   = "etc.log.timeFormat"
)

var defaultTerminals = []Terminal{TerminalConsole, TerminalFile}

type Option func(o *options)

type options struct {
	level        Level    // 输出级别
	syncers      []Syncer // 日志同步器
	terminals    any      // 输出终端
	stackLevel   Level    // 输出栈的日志级别
	callDepth    int      // 输出栈的深度
	callFullPath bool     // 输出栈的调用文件全路径
	timeFormat   string   // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
}

func defaultOptions() *options {
	opts := &options{
		level:        Level(etc.Get(defaultLevelKey, defaultLevel).String()),
		terminals:    defaultTerminals,
		stackLevel:   Level(etc.Get(defaultStackLevelKey, defaultStackLevel).String()),
		callDepth:    etc.Get(defaultCallDepthKey, defaultCallDepth).Int(),
		callFullPath: etc.Get(defaultCallFullPathKey, defaultCallFullPath).Bool(),
		timeFormat:   etc.Get(defaultTimeFormatKey, defaultTimeFormat).String(),
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
	return func(o *options) { o.syncers = syncers }
}

// WithTerminals 设置日志的输出终端
func WithTerminals[T Terminal | []Terminal | map[Terminal][]Level](terminals ...T) Option {
	return func(o *options) { o.terminals = terminals }
}

// WithStackLevel 设置日志的输出栈的日志级别
func WithStackLevel(level Level) Option {
	return func(o *options) { o.stackLevel = level }
}

// WithCallDepth 设置日志的输出栈的深度
func WithCallDepth(depth int) Option {
	return func(o *options) { o.callDepth = depth }
}

// WithCallFullPath 设置日志的输出栈的调用文件全路径
func WithCallFullPath(fullPath bool) Option {
	return func(o *options) { o.callFullPath = fullPath }
}

// WithTimeFormat 设置日志输出时间格式
func WithTimeFormat(timeFormat string) Option {
	return func(o *options) { o.timeFormat = timeFormat }
}
