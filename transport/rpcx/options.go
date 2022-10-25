package rpcx

import (
	"github.com/dobyte/due/config"
)

const (
	defaultServerAddr = ":8661" // 默认服务器地址
)

const (
	defaultServerAddrKey = "config.transport.rpcx.server.addr"
)

type Option func(o *options)

type options struct {
	server struct {
		addr string // 地址
	}
	client struct {
	}
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.addr = config.Get(defaultServerAddrKey, defaultServerAddr).String()

	return opts
}

// WithServerListenAddr 设置RPC服务器监听地址
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.server.addr = addr }
}
