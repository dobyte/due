/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 6:08 下午
 * @Desc: TODO
 */

package tencent

import (
	"github.com/symsimmy/due/config"
	"github.com/symsimmy/due/log"
)

const (
	defaultLevel          = log.InfoLevel
	defaultStdout         = true
	defaultSyncout        = true
	defaultTimeFormat     = "2006/01/02 15:04:05.000000"
	defaultCallerFullPath = true
)

const (
	defaultLevelKey          = "config.log.level"
	defaultTimeFormatKey     = "config.log.timeFormat"
	defaultStackLevelKey     = "config.log.stackLevel"
	defaultStdoutKey         = "config.log.stdout"
	defaultSyncoutKey        = "config.log.syncout"
	defaultCallerFullPathKey = "config.log.callerFullPath"
)

const (
	tencentEndpointKey        = "config.log.tencent.endpoint"
	tencentAccessKeyIDKey     = "config.log.tencent.accessKeyID"
	tencentAccessKeySecretKey = "config.log.tencent.accessKeySecret"
	tencentTopicIDKey         = "config.log.tencent.topicID"
	tencentLevelKey           = "config.log.tencent.level"
	tencentTimeFormatKey      = "config.log.tencent.timeFormat"
	tencentStackLevelKey      = "config.log.tencent.stackLevel"
	tencentStdoutKey          = "config.log.tencent.stdout"
	tencentSyncoutKey         = "config.log.tencent.syncout"
	tencentCallerFullPathKey  = "config.log.tencent.callerFullPath"
)

type Option func(o *options)

type options struct {
	topicID         string // 腾讯云CLS主题ID
	endpoint        string // 腾讯云CLS服务域名，公网使用公网域名，内网使用私网域名
	accessKeyID     string // 腾讯云CLS访问密钥ID
	accessKeySecret string // 腾讯云CLS访问密钥密码

	stdout         bool      // 是否输出到终端，debug模式下默认输出到终端
	syncout        bool      // 是否同步输出到远端，debug模式下默认不输出到远端
	level          log.Level // 输出的最低日志级别，默认Info
	stackLevel     log.Level // 堆栈的最低输出级别，默认不输出堆栈
	timeFormat     string    // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
	callerSkip     int       // 调用者跳过的层级深度，默认为0
	callerFullPath bool      // 是否启用调用文件全路径，默认全路径
}

func defaultOptions() *options {
	opts := &options{
		level:          defaultLevel,
		stdout:         defaultStdout,
		syncout:        defaultSyncout,
		timeFormat:     defaultTimeFormat,
		callerFullPath: defaultCallerFullPath,
	}

	level := config.Get(tencentLevelKey, config.Get(defaultLevelKey).String()).String()
	if lvl := log.ParseLevel(level); lvl != log.NoneLevel {
		opts.level = lvl
	}

	timeFormat := config.Get(tencentTimeFormatKey, config.Get(defaultTimeFormatKey).String()).String()
	if timeFormat != "" {
		opts.timeFormat = timeFormat
	}

	stackLevel := config.Get(tencentStackLevelKey, config.Get(defaultStackLevelKey).String()).String()
	if lvl := log.ParseLevel(stackLevel); lvl != log.NoneLevel {
		opts.stackLevel = lvl
	}

	opts.stdout = config.Get(tencentStdoutKey, config.Get(defaultStdoutKey, defaultStdout).Bool()).Bool()
	opts.syncout = config.Get(tencentSyncoutKey, config.Get(defaultSyncoutKey, defaultSyncout).Bool()).Bool()
	opts.callerFullPath = config.Get(tencentCallerFullPathKey, config.Get(defaultCallerFullPathKey, defaultCallerFullPath).Bool()).Bool()
	opts.endpoint = config.Get(tencentEndpointKey).String()
	opts.accessKeyID = config.Get(tencentAccessKeyIDKey).String()
	opts.accessKeySecret = config.Get(tencentAccessKeySecretKey).String()
	opts.topicID = config.Get(tencentTopicIDKey).String()

	return opts
}

// WithTopicID 设置主题ID
func WithTopicID(topicID string) Option {
	return func(o *options) { o.topicID = topicID }
}

// WithEndpoint 设置端口
func WithEndpoint(endpoint string) Option {
	return func(o *options) { o.endpoint = endpoint }
}

// WithAccessKeyID 设置访问密钥ID
func WithAccessKeyID(accessKeyID string) Option {
	return func(o *options) { o.accessKeyID = accessKeyID }
}

// WithAccessKeySecret 设置访问密钥密码
func WithAccessKeySecret(accessKeySecret string) Option {
	return func(o *options) { o.accessKeySecret = accessKeySecret }
}

// WithStdout 设置是否输出到终端
func WithStdout(enable bool) Option {
	return func(o *options) { o.stdout = enable }
}

// WithSyncout 设置是否同步输出到远端
func WithSyncout(enable bool) Option {
	return func(o *options) { o.syncout = enable }
}

// WithLevel 设置输出的最低日志级别
func WithLevel(level log.Level) Option {
	return func(o *options) { o.level = level }
}

// WithStackLevel 设置堆栈的最小输出级别
func WithStackLevel(level log.Level) Option {
	return func(o *options) { o.stackLevel = level }
}

// WithTimeFormat 设置时间格式
func WithTimeFormat(format string) Option {
	return func(o *options) { o.timeFormat = format }
}

// WithCallerSkip 设置调用者跳过的层级深度
func WithCallerSkip(skip int) Option {
	return func(o *options) { o.callerSkip = skip }
}

// WithCallerFullPath 设置是否启用调用文件全路径
func WithCallerFullPath(enable bool) Option {
	return func(o *options) { o.callerFullPath = enable }
}
