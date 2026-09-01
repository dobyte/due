package polaris

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	"github.com/polarismesh/polaris-go/api"
)

const (
	defaultMode      = config.ReadOnly
	defaultUrl       = "127.0.0.1:8091"
	defaultNamespace = "default"
	defaultGroup     = "default"
	defaultTimeout   = "3s"
	defaultProtocol  = "grpc"
)

const (
	defaultModeKey      = "etc.config.polaris.mode"
	defaultUrlsKey      = "etc.config.polaris.urls"
	defaultNamespaceKey = "etc.config.polaris.namespace"
	defaultGroupKey     = "etc.config.polaris.group"
	defaultTimeoutKey   = "etc.config.polaris.timeout"
	defaultProtocolKey  = "etc.config.polaris.protocol"
)

// Option 配置项
type Option func(o *options)

// options 配置选项
type options struct {
	// 上下文
	// 默认context.Background
	ctx context.Context

	// 读写模式
	// 支持read-only、write-only和read-write三种模式，默认为read-only模式
	mode config.Mode

	// 服务器地址 ip:port
	// 默认为[]string{127.0.0.1:8091}
	urls []string

	// 外部SDK上下文
	// 外部SDK上下文配置，存在外部SDK上下文时，优先使用外部SDK上下文，默认为nil
	client api.SDKContext

	// 命名空间
	// 默认为default
	namespace string

	// 配置分组
	// 默认为default
	group string

	// 请求Polaris服务端超时时间
	// 默认为3秒
	timeout time.Duration

	// 与Polaris服务端的通信协议
	// 默认为grpc
	protocol string
}

// defaultOptions 获取默认配置项
// @return @1 *options 默认配置项
func defaultOptions() *options {
	return &options{
		ctx:       context.Background(),
		mode:      config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
		urls:      etc.Get(defaultUrlsKey, []string{defaultUrl}).Strings(),
		namespace: etc.Get(defaultNamespaceKey, defaultNamespace).String(),
		group:     etc.Get(defaultGroupKey, defaultGroup).String(),
		timeout:   etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		protocol:  etc.Get(defaultProtocolKey, defaultProtocol).String(),
	}
}

// WithContext 设置上下文
// @param ctx context.Context 上下文
// @return @1 Option 配置项
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithMode 设置读写模式
// @param mode config.Mode 读写模式
// @return @1 Option 配置项
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}

// WithUrls 设置服务器地址
// @param urls ...string 服务器地址列表
// @return @1 Option 配置项
func WithUrls(urls ...string) Option {
	return func(o *options) { o.urls = urls }
}

// WithClient 设置外部SDK上下文
// 传入外部SDK上下文时，配置源的构建将跳过客户端构建流程，
// 且该SDK上下文的销毁由调用方负责
// @param client api.SDKContext 外部SDK上下文
// @return @1 Option 配置项
func WithClient(client api.SDKContext) Option {
	return func(o *options) { o.client = client }
}

// WithNamespace 设置命名空间
// @param namespace string 命名空间
// @return @1 Option 配置项
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithGroup 设置配置分组
// @param group string 配置分组
// @return @1 Option 配置项
func WithGroup(group string) Option {
	return func(o *options) { o.group = group }
}

// WithTimeout 设置请求Polaris服务端超时时间
// @param timeout time.Duration 请求超时时间
// @return @1 Option 配置项
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithProtocol 设置与Polaris服务端的通信协议
// @param protocol string 通信协议
// @return @1 Option 配置项
func WithProtocol(protocol string) Option {
	return func(o *options) { o.protocol = protocol }
}
