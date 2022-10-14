package log

import (
	"github.com/dobyte/due/config"
	"strings"
	"time"
)

const (
	defaultOutFile         = "./log/due.log"
	defaultOutLevel        = InfoLevel
	defaultOutFormat       = TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultFileMaxSize     = 100
	defaultFileCutRule     = CutByDay
	defaultCallerFormat    = CallerFullPath
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

type options struct {
	outFile              string        // 输出的文件路径，有文件路径才会输出到文件，否则只会输出到终端
	outLevel             Level         // 输出的最低日志级别，默认Info
	outFormat            Format        // 输出的日志格式，Text或者Json，默认Text
	stackLevel           Level         // 堆栈的最低输出级别，默认不输出堆栈
	callerFormat         CallerFormat  // 调用者格式，默认短路径
	timestampFormat      string        // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
	fileMaxAge           time.Duration // 文件最大留存时间，默认7天
	fileMaxSize          int64         // 文件最大尺寸限制，单位（MB），默认100MB
	fileCutRule          CutRule       // 文件切割规则，默认按照天
	enableLeveledStorage bool          // 是否启用分级存储，默认不分级
	callerSkip           int           // 调用者跳过的层级深度
}

type Option func(o *options)

func defaultOptions() *options {
	opts := &options{
		outFile:         defaultOutFile,
		outLevel:        defaultOutLevel,
		outFormat:       defaultOutFormat,
		fileMaxAge:      defaultFileMaxAge,
		fileMaxSize:     defaultFileMaxSize,
		fileCutRule:     defaultFileCutRule,
		callerFormat:    defaultCallerFormat,
		timestampFormat: defaultTimestampFormat,
	}

	file := config.Get("config.log.file").String()
	if file != "" {
		opts.outFile = file
	}

	level := config.Get("config.log.level").String()
	switch strings.ToUpper(level) {
	case DebugLevel.String():
		opts.outLevel = DebugLevel
	case InfoLevel.String():
		opts.outLevel = InfoLevel
	case WarnLevel.String():
		opts.outLevel = WarnLevel
	case ErrorLevel.String():
		opts.outLevel = ErrorLevel
	case FatalLevel.String():
		opts.outLevel = FatalLevel
	case PanicLevel.String():
		opts.outLevel = PanicLevel
	}

	format := config.Get("config.log.format").String()
	switch strings.ToLower(format) {
	case "json":
		opts.outFormat = JsonFormat
	case "text":
		opts.outFormat = TextFormat
	}

	stackLevel := config.Get("config.log.stackLevel").String()
	switch strings.ToUpper(stackLevel) {
	case DebugLevel.String():
		opts.stackLevel = DebugLevel
	case InfoLevel.String():
		opts.stackLevel = InfoLevel
	case WarnLevel.String():
		opts.stackLevel = WarnLevel
	case ErrorLevel.String():
		opts.stackLevel = ErrorLevel
	case FatalLevel.String():
		opts.stackLevel = FatalLevel
	case PanicLevel.String():
		opts.stackLevel = PanicLevel
	}

	fileMaxAge := config.Get("config.log.fileMaxAge").Duration()
	if fileMaxAge > 0 {
		opts.fileMaxAge = fileMaxAge
	}

	fileMaxSize := config.Get("config.log.fileMaxSize").Int64()
	if fileMaxSize > 0 {
		opts.fileMaxSize = fileMaxSize
	}

	fileCutRule := config.Get("config.log.fileCutRule").String()
	switch strings.ToLower(fileCutRule) {
	case "year":
		opts.fileCutRule = CutByYear
	case "month":
		opts.fileCutRule = CutByMonth
	case "day":
		opts.fileCutRule = CutByDay
	case "hour":
		opts.fileCutRule = CutByHour
	case "minute":
		opts.fileCutRule = CutByMinute
	case "second":
		opts.fileCutRule = CutBySecond
	}

	callerShortPath := config.Get("config.log.callerShortPath").Bool()
	if callerShortPath {
		opts.callerFormat = CallerShortPath
	} else {
		opts.callerFormat = CallerFullPath
	}

	timestampFormat := config.Get("config.log.timestampFormat").String()
	if timestampFormat != "" {
		opts.timestampFormat = timestampFormat
	}

	opts.enableLeveledStorage = config.Get("config.log.leveledStorage").Bool()

	return opts
}

// WithOutFile 设置输出的文件路径
func WithOutFile(file string) Option {
	return func(o *options) { o.outFile = file }
}

// WithOutLevel 设置输出的最低日志级别
func WithOutLevel(level Level) Option {
	return func(o *options) { o.outLevel = level }
}

// WithOutFormat 设置输出的日志格式
func WithOutFormat(format Format) Option {
	return func(o *options) { o.outFormat = format }
}

// WithStackLevel 设置堆栈的最小输出级别
func WithStackLevel(level Level) Option {
	return func(o *options) { o.stackLevel = level }
}

// WithCallerFormat 设置调用者格式
func WithCallerFormat(format CallerFormat) Option {
	return func(o *options) { o.callerFormat = format }
}

// WithTimestampFormat 设置时间格式
func WithTimestampFormat(format string) Option {
	return func(o *options) { o.timestampFormat = format }
}

// WithFileMaxAge 设置文件最大留存时间
func WithFileMaxAge(maxAge time.Duration) Option {
	return func(o *options) { o.fileMaxAge = maxAge }
}

// WithFileMaxSize 设置输出的单个文件尺寸限制
func WithFileMaxSize(size int64) Option {
	return func(o *options) { o.fileMaxSize = size }
}

// WithFileCutRule 设置文件切割规则
func WithFileCutRule(cutRule CutRule) Option {
	return func(o *options) { o.fileCutRule = cutRule }
}

// WithEnableLeveledStorage 设置启用文件分级存储
// 启用后，日志将进行分级存储，大一级的日志将存储于小于等于自身的日志级别文件中
// 例如：InfoLevel级的日志将存储于due.debug.20220910.log、due.info.20220910.log两个日志文件中
func WithEnableLeveledStorage(enable bool) Option {
	return func(o *options) { o.enableLeveledStorage = enable }
}

// WithCallerSkip 设置调用者跳过的层级深度
func WithCallerSkip(skip int) Option {
	return func(o *options) { o.callerSkip = skip }
}
