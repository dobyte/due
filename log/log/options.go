package log

import (
	"time"

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
	defaultFilePath          = "./log/due.log"
	defaultFileMaxAge        = "7d"
	defaultFileMaxSize       = "100M"
	defaultFileRotate        = FileRotateDay
	defaultFileCompress      = true
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
	defaultFilePathKey          = "etc.log.filePath"
	defaultFileMaxAgeKey        = "etc.log.fileMaxAge"
	defaultFileMaxSizeKey       = "etc.log.fileMaxSize"
	defaultFileRotateKey        = "etc.log.fileRotate"
	defaultFileCompressKey      = "etc.log.fileCompress"
)

type Option func(o *options)

type options struct {
	outLevel          Level         // 输出级别
	outFormat         Format        // 输出格式
	outTerminals      []Terminal    // 输出终端
	outStackLevel     Level         // 输出栈的日志级别
	outCallerDepth    int           // 输出栈的深度
	outCallerFullPath bool          // 输出栈的调用文件全路径
	timeZone          string        // 时间时区，默认为Local
	timeFormat        string        // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
	filePath          string        // 文件路径
	fileMaxAge        time.Duration // 文件最大留存时间
	fileMaxSize       int64         // 单个文件最大尺寸
	fileRotate        FileRotate    // 文件反转规则
	fileCompress      bool          // 是否对轮换的日志文件进行压缩
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
		filePath:          etc.Get(defaultFilePathKey, defaultFilePath).String(),
		fileMaxAge:        etc.Get(defaultFileMaxAgeKey, defaultFileMaxAge).Duration(),
		fileMaxSize:       int64(etc.Get(defaultFileMaxSizeKey, defaultFileMaxSize).B()),
		fileRotate:        FileRotate(etc.Get(defaultFileRotateKey, defaultFileRotate).String()),
		fileCompress:      etc.Get(defaultFileCompressKey, defaultFileCompress).Bool(),
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

// WithFilePath 设置文件路径
func WithFilePath(filePath string) Option {
	return func(o *options) { o.filePath = filePath }
}

// WithFileMaxAge 设置文件最大留存时间
func WithFileMaxAge(fileMaxAge time.Duration) Option {
	return func(o *options) { o.fileMaxAge = fileMaxAge }
}

// WithFileMaxSize 设置单个文件最大尺寸
func WithFileMaxSize(fileMaxSize int64) Option {
	return func(o *options) { o.fileMaxSize = fileMaxSize }
}

// WithFileRotate 设置文件反转规则
func WithFileRotate(fileRotate FileRotate) Option {
	return func(o *options) { o.fileRotate = fileRotate }
}

// WithFileCompress 设置是否对轮换日志文件进行压缩
func WithFileCompress(fileCompress bool) Option {
	return func(o *options) { o.fileCompress = fileCompress }
}
