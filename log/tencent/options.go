package tencent

import (
	"github.com/dobyte/due/v2/etc"
)

const (
	tencentEndpointKey        = "etc.log.tencent.endpoint"
	tencentAccessKeyIDKey     = "etc.log.tencent.accessKeyID"
	tencentAccessKeySecretKey = "etc.log.tencent.accessKeySecret"
	tencentTopicIDKey         = "etc.log.tencent.topicID"
)

type Option func(o *options)

type options struct {
	topicID         string // 腾讯云CLS主题ID
	endpoint        string // 腾讯云CLS服务域名，公网使用公网域名，内网使用私网域名
	accessKeyID     string // 腾讯云CLS访问密钥ID
	accessKeySecret string // 腾讯云CLS访问密钥密码
}

func defaultOptions() *options {
	return &options{
		topicID:         etc.Get(tencentTopicIDKey).String(),
		endpoint:        etc.Get(tencentEndpointKey).String(),
		accessKeyID:     etc.Get(tencentAccessKeyIDKey).String(),
		accessKeySecret: etc.Get(tencentAccessKeySecretKey).String(),
	}
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
