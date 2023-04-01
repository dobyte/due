package rpcx

import (
	"github.com/dobyte/due/config"
	"github.com/dobyte/due/transport/rpcx/internal/server"
)

const (
	defaultServerAddr = ":8661" // 默认服务器地址
)

const (
	defaultServerAddrKey = "config.transport.rpcx.server.addr"
)

type Option func(o *options)

type options struct {
	server server.Options
	client struct {
	}
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = config.Get(defaultServerAddrKey, defaultServerAddr).String()

	return opts
}

// WithServerListenAddr 设置RPC服务器监听地址
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}
