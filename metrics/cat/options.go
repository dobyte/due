package cat

import (
	"context"
	"github.com/symsimmy/due/config"
)

const (
	defaultAddr   = "127.0.0.1"
	defaultPort   = 8080
	defaultEnable = false
	defaultName   = "eduverse-due-server"
)

const (
	defaultAddrKey   = "config.cat.addr"
	defaultPortKey   = "config.cat.port"
	defaultEnableKey = "config.cat.enable"
	defaultNameKey   = "config.cat.name"
)

type Option func(o *options)

type options struct {
	ctx    context.Context
	name   string
	addr   string
	port   int
	enable bool
}

func defaultOptions() *options {
	return &options{
		ctx:    context.Background(),
		name:   config.Get(defaultNameKey, defaultName).String(),
		addr:   config.Get(defaultAddrKey, defaultAddr).String(),
		port:   config.Get(defaultPortKey, defaultPort).Int(),
		enable: config.Get(defaultEnableKey, defaultEnable).Bool(),
	}
}
