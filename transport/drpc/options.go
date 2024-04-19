package drpc

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/transport/drpc/internal/server"
)

const (
	defaultServerAddr     = ":0" // 默认服务器地址
	defaultClientPoolSize = 10   // 默认客户端连接池大小
)

const (
	defaultServerAddrKey       = "etc.transport.drpc.server.addr"
	defaultServerKeyFileKey    = "etc.transport.drpc.server.keyFile"
	defaultServerCertFileKey   = "etc.transport.drpc.server.certFile"
	defaultClientPoolSizeKey   = "etc.transport.drpc.client.poolSize"
	defaultClientCertFileKey   = "etc.transport.drpc.client.certFile"
	defaultClientServerNameKey = "etc.transport.drpc.client.serverName"
)

type Option func(o *options)

type options struct {
	server server.Options
	//client client.Options
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
