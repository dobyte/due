package memcache

import (
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/etc"
	"time"
)

const (
	defaultAddr          = "127.0.0.1:11211"
	defaultPrefix        = "cache"
	defaultNilValue      = "cache@nil"
	defaultNilExpiration = "10s"
)

const (
	defaultAddrsKey         = "etc.cache.memcache.addrs"
	defaultPrefixKey        = "etc.cache.memcache.prefix"
	defaultNilValueKey      = "etc.cache.memcache.nilValue"
	defaultNilExpirationKey = "etc.cache.memcache.nilExpiration"
)

type Option func(o *options)

type options struct {
	// 客户端连接地址
	// 内建客户端配置，默认为[]string{"127.0.0.1:6379"}
	addrs []string

	// 客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client *memcache.Client

	// 前缀
	// key前缀，默认为cache
	prefix string

	// 空值，默认为cache@nil
	nilValue string

	// 空值过期时间，默认为10s
	nilExpiration time.Duration
}

func defaultOptions() *options {
	return &options{
		addrs:         etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		prefix:        etc.Get(defaultPrefixKey, defaultPrefix).String(),
		nilValue:      etc.Get(defaultNilValueKey, defaultNilValue).String(),
		nilExpiration: etc.Get(defaultNilExpirationKey, defaultNilExpiration).Duration(),
	}
}

// WithAddrs 设置连接地址
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithClient 设置外部客户端
func WithClient(client *memcache.Client) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix 设置前缀
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithNilValue 设置空值
func WithNilValue(nilValue string) Option {
	return func(o *options) { o.nilValue = nilValue }
}

// WithNilExpiration 设置空值过期时间
func WithNilExpiration(nilExpiration time.Duration) Option {
	return func(o *options) { o.nilExpiration = nilExpiration }
}
