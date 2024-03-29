package gnet

import (
	"github.com/symsimmy/due/config"
	"github.com/symsimmy/due/transport/gnet/internal/server"
	"github.com/symsimmy/due/transport/gnet/tcp"
)

const (
	defaultServerAddr = ":0" // 默认服务器地址
)

const (
	defaultServerAddrKey = "config.transport.tcp.server.addr"
)

type Option func(o *options)

type options struct {
	server server.Options
	client tcp.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = config.Get(defaultServerAddrKey, defaultServerAddr).String()

	return opts
}
