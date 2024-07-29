package consul

import (
	"context"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	"github.com/hashicorp/consul/api"
)

const (
	defaultAddr = "127.0.0.1:8500"
	defaultPath = "config"
	defaultMode = config.ReadOnly
)

const (
	defaultAddrKey = "etc.config.consul.addr"
	defaultPathKey = "etc.config.consul.path"
	defaultModeKey = "etc.config.consul.mode"
)

type Option func(o *options)

type options struct {
	// 上下文
	// 默认为context.Background
	ctx context.Context

	// 客户端连接地址
	// 内建客户端配置，默认为127.0.0.1:8500
	addr string

	// 外部客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client *api.Client

	// 路径
	// 默认为 /config
	path string

	// 读写模式
	// 支持read-only、write-only和read-write三种模式，默认为read-only模式
	mode config.Mode
}

func defaultOptions() *options {
	return &options{
		ctx:  context.Background(),
		addr: etc.Get(defaultAddrKey, defaultAddr).String(),
		path: etc.Get(defaultPathKey, defaultPath).String(),
		mode: config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
	}
}

// WithAddr 设置客户端连接地址
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithClient 设置外部客户端
func WithClient(client *api.Client) Option {
	return func(o *options) { o.client = client }
}

// WithContext 设置context
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithPath 设置基础路径
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode 设置读写模式
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}
