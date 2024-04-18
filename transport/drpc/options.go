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
	//client client.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = etc.Get(defaultServerAddrKey, defaultServerAddr).String()

	return opts
}
