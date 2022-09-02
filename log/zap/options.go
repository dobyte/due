/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 4:56 下午
 * @Desc: TODO
 */

package zap

import (
	"time"

	"github.com/dobyte/due/log"
)

type options struct {
	outFile             string        // 输出的文件路径
	outLevel            log.Level     // 输出的最低日志级别，默认Warn
	outFormat           log.Format    // 输出的日志格式，默认Text
	fileMaxAge          time.Duration // 文件最大留存时间，单位（）默认7天
	fileMaxSize         int64         // 文件最大尺寸限制，单位（MB），默认100MB
	fileCutRule         log.CutRule   // 文件切割规则，默认按照天
	fileClassifyStorage bool          // 文件分级存储，默认统一存储
	timestampFormat     string        // 日志时间戳格式，标准库时间格式，默认2006/01/02 15:04:05.000000
	callerFullPath      bool          // 是否显示调用者全路径，默认短路径
}

type Option func(o *options)

// 设置输出的文件路径
func WithOutFile(file string) Option {
	return func(o *options) { o.outFile = file }
}

// 设置输出的最低日志级别
func WithOutLevel(level log.Level) Option {
	return func(o *options) { o.outLevel = level }
}

// 设置输出的日志格式
func WithOutFormat(format log.Format) Option {
	return func(o *options) { o.outFormat = format }
}

// 设置文件最大留存时间
func WithFileMaxAge(maxAge time.Duration) Option {
	return func(o *options) { o.fileMaxAge = maxAge }
}

// 设置输出的单个文件尺寸限制
func WithFileMaxSize(size int64) Option {
	return func(o *options) { o.fileMaxSize = size }
}

// 设置文件切割规则
func WithFileCutRule(cutRule log.CutRule) Option {
	return func(o *options) { o.fileCutRule = cutRule }
}

// 设置文件分类存储
func WithFileClassifyStorage(enable bool) Option {
	return func(o *options) { o.fileClassifyStorage = enable }
}

// 设置日志输出时间戳格式，标准库时间格式
func WithTimestampFormat(format string) Option {
	return func(o *options) { o.timestampFormat = format }
}

// 设置是否显示调用者全路径
func WithCallerFullPath(callerFullPath bool) Option {
	return func(o *options) { o.callerFullPath = callerFullPath }
}
