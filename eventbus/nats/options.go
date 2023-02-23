package nats

import (
	"context"
	"github.com/dobyte/due/config"
	"github.com/nats-io/nats.go"
)

const (
	defaultUrl        = "nats://127.0.0.1:4222"
	defaultDB         = 0
	defaultMaxRetries = 3
	defaultPrefix     = "due"
)

const (
	defaultUrlKey        = "config.eventbus.nats.url"
	defaultDBKey         = "config.eventbus.nats.db"
	defaultMaxRetriesKey = "config.eventbus.nats.maxRetries"
	defaultPrefixKey     = "config.eventbus.nats.prefix"
	defaultUsernameKey   = "config.eventbus.nats.username"
	defaultPasswordKey   = "config.eventbus.nats.password"
)

type Option func(o *options)

type options struct {
	ctx context.Context

	// 客户端连接地址
	// 内建客户端配置，默认为nats://127.0.0.1:4222
	url string

	// 客户端连接
	// 外部客户端连接配置，存在外部客户端连接时，优先使用外部客户端连接，默认为nil
	conn *nats.Conn
}

func defaultOptions() *options {
	return &options{
		ctx: context.Background(),
		url: config.Get(defaultUrlKey, defaultUrl).String(),
	}
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithUrl 设置连接地址
func WithUrl(url string) Option {
	return func(o *options) { o.url = url }
}

// WithConn 设置外部客户端连接
func WithConn(conn *nats.Conn) Option {
	return func(o *options) { o.conn = conn }
}
