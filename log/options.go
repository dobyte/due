package log

import (
	"reflect"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultLevel        = LevelInfo
	defaultStackLevel   = LevelError
	defaultTimeFormat   = "2006/01/02 15:04:05.000000"
	defaultCallSkip     = 2
	defaultCallFullPath = false
)

const (
	defaultLevelKey        = "etc.log.level"
	defaultTerminalsKey    = "etc.log.terminals"
	defaultStackLevelKey   = "etc.log.stackLevel"
	defaultTimeFormatKey   = "etc.log.timeFormat"
	defaultCallSkipKey     = "etc.log.callSkip"
	defaultCallFullPathKey = "etc.log.callFullPath"
)

var defaultTerminals = []Terminal{TerminalConsole, TerminalFile}

type Option func(o *options)

type options struct {
	level        Level    // 输出级别
	syncers      []Syncer // 日志同步器
	terminals    any      // 输出终端
	stackLevel   Level    // 输出栈的日志级别
	callSkip     int      // 输出栈的跳过深度
	callFullPath bool     // 输出栈的调用文件全路径
	timeFormat   string   // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
}

func defaultOptions() *options {
	opts := &options{
		level:        Level(etc.Get(defaultLevelKey, defaultLevel).String()),
		terminals:    defaultTerminals,
		stackLevel:   Level(etc.Get(defaultStackLevelKey, defaultStackLevel).String()),
		timeFormat:   etc.Get(defaultTimeFormatKey, defaultTimeFormat).String(),
		callSkip:     etc.Get(defaultCallSkipKey, defaultCallSkip).Int(),
		callFullPath: etc.Get(defaultCallFullPathKey, defaultCallFullPath).Bool(),
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

// WithTimeFormat 设置日志输出时间格式
func WithTimeFormat(timeFormat string) Option {
	return func(o *options) { o.timeFormat = timeFormat }
}

// WithCallSkip 设置输出栈的跳过深度
func WithCallSkip(skip int) Option {
	return func(o *options) { o.callSkip = skip }
}

// WithCallFullPath 设置日志的输出栈的调用文件全路径
func WithCallFullPath(fullPath bool) Option {
	return func(o *options) { o.callFullPath = fullPath }
}
