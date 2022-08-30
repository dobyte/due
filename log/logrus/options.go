/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/29 7:27 下午
 * @Desc: TODO
 */

package logrus

import "github.com/dobyte/due/log"

type options struct {
	level           log.Level // 最低日志级别
	format          string    // 日志输出格式
	timestampFormat string    // 日志输出时间戳格式，标准库时间格式
}

type Option func(o *options)

// 设置最低日志级别
func WithLevel(level log.Level) Option {
	return func(o *options) { o.level = level }
}

// 设置日志输出格式
func WithFormat(format string) Option {
	return func(o *options) { o.format = format }
}

// 设置日志输出时间戳格式，标准库时间格式
func WithTimestampFormat(format string) Option {
	return func(o *options) { o.timestampFormat = format }
}
