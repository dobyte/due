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

// Option configures the syncer.
type Option func(o *options)

type options struct {
	topicID         string // Tencent Cloud CLS topic ID
	endpoint        string // Tencent Cloud CLS service endpoint; use the public domain over the internet and the private domain within a VPC
	accessKeyID     string // Tencent Cloud CLS access key ID
	accessKeySecret string // Tencent Cloud CLS access key secret
}

func defaultOptions() *options {
	return &options{
		topicID:         etc.Get(tencentTopicIDKey).String(),
		endpoint:        etc.Get(tencentEndpointKey).String(),
		accessKeyID:     etc.Get(tencentAccessKeyIDKey).String(),
		accessKeySecret: etc.Get(tencentAccessKeySecretKey).String(),
	}
}

// WithTopicID sets the Tencent Cloud CLS topic ID.
func WithTopicID(topicID string) Option {
	return func(o *options) { o.topicID = topicID }
}

// WithEndpoint sets the Tencent Cloud CLS service endpoint.
func WithEndpoint(endpoint string) Option {
	return func(o *options) { o.endpoint = endpoint }
}

// WithAccessKeyID sets the Tencent Cloud CLS access key ID.
func WithAccessKeyID(accessKeyID string) Option {
	return func(o *options) { o.accessKeyID = accessKeyID }
}

// WithAccessKeySecret sets the Tencent Cloud CLS access key secret.
func WithAccessKeySecret(accessKeySecret string) Option {
	return func(o *options) { o.accessKeySecret = accessKeySecret }
}
