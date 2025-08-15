package log

import (
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultOutLevel          = LevelInfo
	defaultOutFormat         = FormatText
	defaultOutStackLevel     = LevelError
	defaultOutCallerDepth    = 0
	defaultOutCallerFullPath = false
	defaultTimeZone          = "Local"
	defaultTimeFormat        = "2006/01/02 15:04:05.000000"
)

const (
	defaultOutLevelKey          = "etc.log.outLevel"
	defaultOutFormatKey         = "etc.log.outFormat"
	defaultOutTerminalsKey      = "etc.log.outTerminals"
	defaultOutStackLevelKey     = "etc.log.outStackLevel"
	defaultOutCallerDepthKey    = "etc.log.outCallerDepth"
	defaultOutCallerFullPathKey = "etc.log.outCallerFullPath"
	defaultTimeZoneKey          = "etc.log.timeZone"
	defaultTimeFormatKey        = "etc.log.timeFormat"
)

type Option func(o *options)

type options struct {
	outLevel          Level      // 输出级别
	outFormat         Format     // 输出格式
	outTerminals      []Terminal // 输出终端
	outStackLevel     Level      // 输出栈的日志级别
	outCallerDepth    int        // 输出栈的深度
	outCallerFullPath bool       // 输出栈的调用文件全路径
	timeZone          string     // 时间时区，默认为Local
	timeFormat        string     // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
}

func defaultOptions() *options {
	opts := &options{
		outLevel:          Level(etc.Get(defaultOutLevelKey, defaultOutLevel).String()),
		outFormat:         Format(etc.Get(defaultOutFormatKey, defaultOutFormat).String()),
		outTerminals:      make([]Terminal, 0, 2),
		outStackLevel:     Level(etc.Get(defaultOutStackLevelKey, defaultOutStackLevel).String()),
		outCallerDepth:    etc.Get(defaultOutCallerDepthKey, defaultOutCallerDepth).Int(),
		outCallerFullPath: etc.Get(defaultOutCallerFullPathKey, defaultOutCallerFullPath).Bool(),
		timeZone:          etc.Get(defaultTimeZoneKey, defaultTimeZone).String(),
		timeFormat:        etc.Get(defaultTimeFormatKey, defaultTimeFormat).String(),
	}

	if err := etc.Get(defaultOutTerminalsKey).Scan(&opts.outTerminals); err != nil || len(opts.outTerminals) == 0 {
		opts.outTerminals = []Terminal{TerminalConsole, TerminalFile}
	}

	return opts
}

// WithOutLevel 设置日志的输出级别
func WithOutLevel(level Level) Option {
	return func(o *options) { o.outLevel = level }
}

// WithOutFormat 设置日志的输出格式
func WithOutFormat(format Format) Option {
	return func(o *options) { o.outFormat = format }
}

// WithOutTerminal 设置日志的输出终端
func WithOutTerminal(terminals ...Terminal) Option {
	return func(o *options) { o.outTerminals = terminals }
}

// WithOutStackLevel 设置日志的输出栈的日志级别
func WithOutStackLevel(level Level) Option {
	return func(o *options) { o.outStackLevel = level }
}

// WithOutStackDepth 设置日志的输出栈的深度
func WithOutCallerDepth(depth int) Option {
	return func(o *options) { o.outCallerDepth = depth }
}

// WithOutCallerFullPath 设置日志的输出栈的调用文件全路径
func WithOutCallerFullPath(fullPath bool) Option {
	return func(o *options) { o.outCallerFullPath = fullPath }
}

// WithTimeZone 设置日志文件打印时间的时区
func WithTimeZone(timeZone string) Option {
	return func(o *options) { o.timeZone = timeZone }
}

// WithTimeFormat 设置日志输出时间格式
func WithTimeFormat(timeFormat string) Option {
	return func(o *options) { o.timeFormat = timeFormat }
}
