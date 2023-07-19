package grpc

import (
	"github.com/dobyte/due/transport/grpc/v2/internal/client"
	"github.com/dobyte/due/transport/grpc/v2/internal/server"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc"
)

const (
	defaultServerAddr     = ":0" // 默认服务器地址
	defaultClientPoolSize = 10   // 默认客户端连接池大小
)

const (
	defaultServerAddrKey       = "config.transport.grpc.server.addr"
	defaultServerKeyFileKey    = "config.transport.grpc.server.keyFile"
	defaultServerCertFileKey   = "config.transport.grpc.server.certFile"
	defaultClientPoolSizeKey   = "config.transport.grpc.client.poolSize"
	defaultClientCertFileKey   = "config.transport.grpc.client.certFile"
	defaultClientServerNameKey = "config.transport.grpc.client.serverName"
)

type Option func(o *options)

type options struct {
	server server.Options
	client client.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = config.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.server.KeyFile = config.Get(defaultServerKeyFileKey).String()
	opts.server.CertFile = config.Get(defaultServerCertFileKey).String()
	opts.client.PoolSize = config.Get(defaultClientPoolSizeKey, defaultClientPoolSize).Int()
	opts.client.CertFile = config.Get(defaultClientCertFileKey).String()
	opts.client.ServerName = config.Get(defaultClientServerNameKey).String()

	return opts
}

// WithServerListenAddr 设置服务器监听地址
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}

// WithServerCredentials 设置服务器证书和秘钥
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.server.KeyFile, o.server.CertFile = keyFile, certFile }
}

// WithServerOptions 设置服务器选项
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) { o.server.ServerOpts = opts }
}

// WithClientPoolSize 设置客户端连接池大小
func WithClientPoolSize(size int) Option {
	return func(o *options) { o.client.PoolSize = size }
}

// WithClientCredentials 设置客户端证书和校验域名
func WithClientCredentials(certFile string, serverName string) Option {
	return func(o *options) { o.client.CertFile, o.client.ServerName = certFile, serverName }
}

// WithClientDiscovery 设置客户端服务发现组件
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) { o.client.Discovery = discovery }
}

// WithClientDialOptions 设置客户端拨号选项
func WithClientDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) { o.client.DialOpts = opts }
}
