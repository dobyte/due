package polaris

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/polarismesh/polaris-go/api"
)

const (
	defaultUrl       = "127.0.0.1:8091"
	defaultNamespace = "default"
	defaultTimeout   = "3s"
	defaultProtocol  = "grpc"
)

const (
	defaultUrlsKey      = "etc.registry.polaris.urls"
	defaultNamespaceKey = "etc.registry.polaris.namespace"
	defaultTimeoutKey   = "etc.registry.polaris.timeout"
	defaultProtocolKey  = "etc.registry.polaris.protocol"
)

type Option func(o *options)

// options Polaris注册中心配置项
type options struct {
	// 服务器地址 ip:port
	// 默认为[]string{127.0.0.1:8091}
	urls []string

	// 外部SDK上下文
	// 外部SDK上下文配置，存在外部SDK上下文时，优先使用外部SDK上下文，默认为nil
	client api.SDKContext

	// 命名空间
	// 默认为default
	namespace string

	// 请求Polaris服务端超时时间
	// 默认为3秒
	timeout time.Duration

	// 与Polaris服务端的通信协议
	// 默认为grpc
	protocol string
}

func defaultOptions() *options {
	return &options{
		urls:      etc.Get(defaultUrlsKey, []string{defaultUrl}).Strings(),
		namespace: etc.Get(defaultNamespaceKey, defaultNamespace).String(),
		timeout:   etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		protocol:  etc.Get(defaultProtocolKey, defaultProtocol).String(),
	}
}

// WithUrls 设置服务器地址
func WithUrls(urls ...string) Option {
	return func(o *options) { o.urls = urls }
}

// WithClient 设置外部SDK上下文
func WithClient(client api.SDKContext) Option {
	return func(o *options) { o.client = client }
}

// WithNamespace 设置命名空间
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithTimeout 设置请求Polaris服务端超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithProtocol 设置与Polaris服务端的通信协议
func WithProtocol(protocol string) Option {
	return func(o *options) { o.protocol = protocol }
}
