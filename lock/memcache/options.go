package memcache

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultAddr              = "127.0.0.1:11211"
	defaultPrefix            = "due:lock"
	defaultExpiration        = "3s"
	defaultAcquireInterval   = "20ms"
	defaultAcquireMaxRetries = 0
)

const (
	defaultAddrsKey             = "etc.lock.memcache.addrs"
	defaultPrefixKey            = "etc.lock.memcache.prefix"
	defaultExpirationKey        = "etc.lock.memcache.expiration"
	defaultAcquireIntervalKey   = "etc.lock.memcache.acquireInterval"
	defaultAcquireMaxRetriesKey = "etc.lock.memcache.acquireMaxRetries"
)

type Option func(o *options)

type options struct {
	// 客户端连接地址
	// 内建客户端配置，默认为[]string{"127.0.0.1:11211"}
	addrs []string

	// 客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client *memcache.Client

	// 前缀
	// key前缀，默认为due:lock
	prefix string

	// 锁过期时间，默认为3s
	expiration time.Duration

	// 循环获取锁的频率间隔时间，默认为20ms
	acquireInterval time.Duration

	// 循环获取锁的最大重试次数，默认为无限次
	acquireMaxRetries int
}

func defaultOptions() *options {
	return &options{
		addrs:             etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		prefix:            etc.Get(defaultPrefixKey, defaultPrefix).String(),
		expiration:        etc.Get(defaultExpirationKey, defaultExpiration).Duration(),
		acquireInterval:   etc.Get(defaultAcquireIntervalKey, defaultAcquireInterval).Duration(),
		acquireMaxRetries: etc.Get(defaultAcquireMaxRetriesKey, defaultAcquireMaxRetries).Int(),
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

// WithExpiration 锁过期时间
func WithExpiration(expiration time.Duration) Option {
	return func(o *options) { o.expiration = expiration }
}

// WithAcquireInterval 设置获取锁的时间间隔
func WithAcquireInterval(acquireInterval time.Duration) Option {
	return func(o *options) { o.acquireInterval = acquireInterval }
}

// WithAcquireMaxRetries 设置循环获取锁的最大重试次数
func WithAcquireMaxRetries(acquireMaxRetries int) Option {
	return func(o *options) { o.acquireMaxRetries = acquireMaxRetries }
}
