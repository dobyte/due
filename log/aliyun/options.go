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

// Option configures the syncer.
type Option func(o *options)

type options struct {
	endpoint        string // Aliyun SLS service endpoint; use the public domain over the internet and the private domain within a VPC
	accessKeyID     string // Aliyun SLS access key ID
	accessKeySecret string // Aliyun SLS access key secret
	project         string // Aliyun SLS project name
	logstore        string // Aliyun SLS logstore
	topic           string // Topic tag, empty by default
	source          string // Source tag, empty by default
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

// WithProject sets the Aliyun SLS project name.
func WithProject(project string) Option {
	return func(o *options) { o.project = project }
}

// WithLogstore sets the Aliyun SLS logstore.
func WithLogstore(logstore string) Option {
	return func(o *options) { o.logstore = logstore }
}

// WithEndpoint sets the Aliyun SLS service endpoint.
func WithEndpoint(endpoint string) Option {
	return func(o *options) { o.endpoint = endpoint }
}

// WithAccessKeyID sets the Aliyun SLS access key ID.
func WithAccessKeyID(accessKeyID string) Option {
	return func(o *options) { o.accessKeyID = accessKeyID }
}

// WithAccessKeySecret sets the Aliyun SLS access key secret.
func WithAccessKeySecret(accessKeySecret string) Option {
	return func(o *options) { o.accessKeySecret = accessKeySecret }
}

// WithTopic sets the topic tag.
func WithTopic(topic string) Option {
	return func(o *options) { o.topic = topic }
}

// WithSource sets the source tag.
func WithSource(source string) Option {
	return func(o *options) { o.source = source }
}
