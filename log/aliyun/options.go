/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 11:31 上午
 * @Desc: TODO
 */

package aliyun

import (
	"github.com/dobyte/due/v2/etc"
)

const (
	aliyunEndpointKey        = "etc.log.aliyun.endpoint"
	aliyunAccessKeyIDKey     = "etc.log.aliyun.accessKeyID"
	aliyunAccessKeySecretKey = "etc.log.aliyun.accessKeySecret"
	aliyunProjectKey         = "etc.log.aliyun.project"
	aliyunLogstoreKey        = "etc.log.aliyun.logstore"
	aliyunTopicKey           = "etc.log.aliyun.topic"
	aliyunSourceKey          = "etc.log.aliyun.source"
)

type Option func(o *options)

type options struct {
	endpoint        string // 阿里云SLS服务域名，公网使用公网域名，内网使用私网域名
	accessKeyID     string // 阿里云SLS访问密钥ID
	accessKeySecret string // 阿里云SLS访问密钥密码
	project         string // 阿里云SLS项目名称
	logstore        string // 阿里云SLS日志存储
	topic           string // 主题标签，默认为空
	source          string // 来源标签，默认为空
}

func defaultOptions() *options {
	return &options{
		endpoint:        etc.Get(aliyunEndpointKey).String(),
		accessKeyID:     etc.Get(aliyunAccessKeyIDKey).String(),
		accessKeySecret: etc.Get(aliyunAccessKeySecretKey).String(),
		project:         etc.Get(aliyunProjectKey).String(),
		logstore:        etc.Get(aliyunLogstoreKey).String(),
		topic:           etc.Get(aliyunTopicKey).String(),
		source:          etc.Get(aliyunSourceKey).String(),
	}
}

// WithProject 设置项目名称
func WithProject(project string) Option {
	return func(o *options) { o.project = project }
}

// WithLogstore 设置日志存储
func WithLogstore(logstore string) Option {
	return func(o *options) { o.logstore = logstore }
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

// WithTopic 设置主题标签
func WithTopic(topic string) Option {
	return func(o *options) { o.topic = topic }
}

// WithSource 设置来源标签
func WithSource(source string) Option {
	return func(o *options) { o.source = source }
}
