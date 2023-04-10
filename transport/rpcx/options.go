package rpcx

import (
	"github.com/dobyte/due/config"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/transport/rpcx/internal/client"
	"github.com/dobyte/due/transport/rpcx/internal/server"
)

const (
	defaultServerAddr = ":0" // 默认服务器地址
)

const (
	defaultServerAddrKey     = "config.transport.rpcx.server.addr"
	defaultServerKeyFileKey  = "config.transport.rpcx.server.keyFile"
	defaultServerCertFileKey = "config.transport.rpcx.server.certFile"
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

// WithClientDiscovery 设置客户端服务发现组件
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) { o.client.Discovery = discovery }
}
