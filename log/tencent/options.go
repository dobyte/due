/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 6:08 下午
 * @Desc: TODO
 */

package tencent

import "github.com/dobyte/due/log"

type Option func(o *options)

type options struct {
	topicID         string           // 腾讯云CLS主题ID
	endpoint        string           // 腾讯云CLS服务域名，公网使用公网域名，内网使用私网域名
	accessKeyID     string           // 腾讯云CLS访问密钥ID
	accessKeySecret string           // 腾讯云CLS访问密钥密码
	outLevel        log.Level        // 输出的最低日志级别，默认Info
	stackLevel      log.Level        // 堆栈的最低输出级别，默认不输出堆栈
	callerFormat    log.CallerFormat // 调用者格式，默认短路径
	timestampFormat string           // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
	callerSkip      int              // 调用者跳过的层级深度，默认为0
	disableSyncing  bool             // 禁止同步到阿里云SLS服务，默认开启同步
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

// WithOutLevel 设置输出的最低日志级别
func WithOutLevel(level log.Level) Option {
	return func(o *options) { o.outLevel = level }
}

// WithStackLevel 设置堆栈的最小输出级别
func WithStackLevel(level log.Level) Option {
	return func(o *options) { o.stackLevel = level }
}

// WithCallerFormat 设置调用者格式
func WithCallerFormat(format log.CallerFormat) Option {
	return func(o *options) { o.callerFormat = format }
}

// WithTimestampFormat 设置时间格式
func WithTimestampFormat(format string) Option {
	return func(o *options) { o.timestampFormat = format }
}

// WithCallerSkip 设置调用者跳过的层级深度
func WithCallerSkip(skip int) Option {
	return func(o *options) { o.callerSkip = skip }
}

// WithDisableSyncing 设置禁止同步到阿里云SLS服务
func WithDisableSyncing(disable bool) Option {
	return func(o *options) { o.disableSyncing = disable }
}
