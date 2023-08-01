package rpcx

import (
	"github.com/dobyte/due/transport/rpcx/v2/internal/client"
	"github.com/dobyte/due/transport/rpcx/v2/internal/server"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
)

const (
	defaultServerAddr     = ":0" // 默认服务器地址
	defaultClientPoolSize = 10   // 默认客户端连接池大小
)

const (
	defaultServerAddrKey       = "etc.transport.rpcx.server.addr"
	defaultServerKeyFileKey    = "etc.transport.rpcx.server.keyFile"
	defaultServerCertFileKey   = "etc.transport.rpcx.server.certFile"
	defaultClientPoolSizeKey   = "etc.transport.rpcx.client.poolSize"
	defaultClientCertFileKey   = "etc.transport.rpcx.client.certFile"
	defaultClientServerNameKey = "etc.transport.rpcx.client.serverName"
)

type Option func(o *options)

type options struct {
	server server.Options
	client client.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = etc.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.server.KeyFile = etc.Get(defaultServerKeyFileKey).String()
	opts.server.CertFile = etc.Get(defaultServerCertFileKey).String()
	opts.client.PoolSize = etc.Get(defaultClientPoolSizeKey, defaultClientPoolSize).Int()
	opts.client.CertFile = etc.Get(defaultClientCertFileKey).String()
	opts.client.ServerName = etc.Get(defaultClientServerNameKey).String()

	return opts
}

// WithServerListenAddr 设置RPC服务器监听地址
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}

// WithServerCredentials 设置服务器证书和秘钥
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.server.KeyFile, o.server.CertFile = keyFile, certFile }
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
