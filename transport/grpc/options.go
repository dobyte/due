package grpc

import (
	"github.com/dobyte/due/transport/grpc/v2/internal/client"
	"github.com/dobyte/due/transport/grpc/v2/internal/server"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc"
)

const (
	defaultServerAddr = ":0" // 默认服务器地址
)

const (
	defaultServerAddrKey       = "etc.transport.grpc.server.addr"
	defaultServerExposeKey     = "etc.transport.grpc.server.expose"
	defaultServerKeyFileKey    = "etc.transport.grpc.server.keyFile"
	defaultServerCertFileKey   = "etc.transport.grpc.server.certFile"
	defaultClientCAFileKey     = "etc.transport.grpc.client.caFile"
	defaultClientServerNameKey = "etc.transport.grpc.client.serverName"
)

type Option func(o *options)

type options struct {
	server server.Options
	client client.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = etc.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.server.Expose = etc.Get(defaultServerExposeKey).Bool()
	opts.server.KeyFile = etc.Get(defaultServerKeyFileKey).String()
	opts.server.CertFile = etc.Get(defaultServerCertFileKey).String()
	opts.client.CAFile = etc.Get(defaultClientCAFileKey).String()
	opts.client.ServerName = etc.Get(defaultClientServerNameKey).String()

	return opts
}

// WithServerAddr 设置服务器监听地址
func WithServerAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}

// WithServerExpose 设置是否将内部通信地址暴露到公网
func WithServerExpose(expose bool) Option {
	return func(o *options) { o.server.Expose = expose }
}

// WithServerCredentials 设置服务器证书和秘钥
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.server.CertFile, o.server.KeyFile = certFile, keyFile }
}

// WithServerOptions 设置服务器选项
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) { o.server.ServerOpts = opts }
}

// WithClientCredentials 设置客户端证书和校验域名
func WithClientCredentials(caFile string, serverName string) Option {
	return func(o *options) { o.client.CAFile, o.client.ServerName = caFile, serverName }
}

// WithClientDiscovery 设置客户端服务发现组件
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) { o.client.Discovery = discovery }
}

// WithClientDialOptions 设置客户端拨号选项
func WithClientDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) { o.client.DialOpts = opts }
}
