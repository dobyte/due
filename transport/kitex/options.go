package kitex

import (
	"github.com/cloudwego/kitex/client"
	"github.com/dobyte/due/transport/kitex/v2/internal/server"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
)

const (
	defaultServerAddr     = ":0" // 默认服务器地址
	defaultClientPoolSize = 10   // 默认客户端连接池大小
)

const (
	defaultServerAddrKey       = "etc.transport.kitex.server.addr"
	defaultServerKeyFileKey    = "etc.transport.kitex.server.keyFile"
	defaultServerCertFileKey   = "etc.transport.kitex.server.certFile"
	defaultClientPoolSizeKey   = "etc.transport.kitex.client.poolSize"
	defaultClientCertFileKey   = "etc.transport.kitex.client.certFile"
	defaultClientServerNameKey = "etc.transport.kitex.client.serverName"
)

type Option func(o *options)

type options struct {
	server server.Options
	client client.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = etc.Get(defaultServerAddrKey, defaultServerAddr).String()

	return opts
}

// WithServerListenAddr 设置RPC服务器监听地址
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}

// WithClientDiscovery 设置客户端服务发现组件
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) {
		//o.client.Discovery = discovery
	}
}
