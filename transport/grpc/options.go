package grpc

import (
	"github.com/dobyte/due/config"
	"google.golang.org/grpc"
)

const (
	defaultServerAddr = ":8661" // 默认服务器地址
)

const (
	defaultServerAddrKey       = "config.transport.grpc.server.addr"
	defaultServerKeyFileKey    = "config.transport.grpc.server.keyFile"
	defaultServerCertFileKey   = "config.transport.grpc.server.certFile"
	defaultClientCertFileKey   = "config.transport.grpc.client.certFile"
	defaultClientServerNameKey = "config.transport.grpc.client.serverName"
)

type Option func(o *options)

type options struct {
	server struct {
		addr       string              // 地址
		certFile   string              // 证书文件
		keyFile    string              // 秘钥文件
		serverOpts []grpc.ServerOption // 服务器选项
	}
	client struct {
		certFile   string // 证书文件
		serverName string // 证书校验域名
	}
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.addr = config.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.server.keyFile = config.Get(defaultServerKeyFileKey).String()
	opts.server.certFile = config.Get(defaultServerCertFileKey).String()
	opts.client.certFile = config.Get(defaultClientCertFileKey).String()
	opts.client.serverName = config.Get(defaultClientServerNameKey).String()

	return opts
}

// WithServerListenAddr 设置RPC服务器监听地址
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.server.addr = addr }
}

// WithServerCredentials 设置RPC服务器证书和秘钥
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.server.keyFile, o.server.certFile = keyFile, certFile }
}

// WithServerOptions 设置网关RPC服务器选项
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) { o.server.serverOpts = opts }
}

// WithClientCredentials 客户端证书和校验域名
func WithClientCredentials(certFile string, serverName string) Option {
	return func(o *options) { o.client.certFile, o.client.serverName = certFile, serverName }
}
