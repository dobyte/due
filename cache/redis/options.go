package redis

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr          = "127.0.0.1:6379"
	defaultDB            = 0
	defaultMaxRetries    = 3
	defaultPrefix        = "due:cache"
	defaultNilValue      = "cache@nil"
	defaultNilExpiration = "10s"
	defaultMinExpiration = "1h"
	defaultMaxExpiration = "24h"
)

const (
	defaultAddrsKey         = "etc.cache.redis.addrs"
	defaultDBKey            = "etc.cache.redis.db"
	defaultMaxRetriesKey    = "etc.cache.redis.maxRetries"
	defaultPrefixKey        = "etc.cache.redis.prefix"
	defaultUsernameKey      = "etc.cache.redis.username"
	defaultPasswordKey      = "etc.cache.redis.password"
	defaultCertFileKey      = "etc.cache.redis.certFile"
	defaultKeyFileKey       = "etc.cache.redis.keyFile"
	defaultCAFileKey        = "etc.cache.redis.caFile"
	defaultNilValueKey      = "etc.cache.redis.nilValue"
	defaultNilExpirationKey = "etc.cache.redis.nilExpiration"
	defaultMinExpirationKey = "etc.cache.redis.minExpiration"
	defaultMaxExpirationKey = "etc.cache.redis.maxExpiration"
)

type Option func(o *options)

type options struct {
	// 客户端连接地址
	// 内建客户端配置，默认为[]string{"127.0.0.1:6379"}
	addrs []string

	// 数据库号
	// 内建客户端配置，默认为0
	db int

	// 用户名
	// 内建客户端配置，默认为空
	username string

	// 密码
	// 内建客户端配置，默认为空
	password string

	// 客户端证书
	certFile string

	// 客户端密钥
	keyFile string

	// CA证书
	caFile string

	// 最大重试次数
	// 内建客户端配置，默认为3次
	maxRetries int

	// 客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client redis.UniversalClient

	// 前缀
	// key前缀，默认为due:cache
	prefix string

	// 空值，默认为cache@nil
	nilValue string

	// 空值过期时间，默认为10s
	nilExpiration time.Duration

	// 最小过期时间，默认为1h
	minExpiration time.Duration

	// 最大过期时间，默认为24h
	maxExpiration time.Duration
}

func defaultOptions() *options {
	return &options{
		addrs:         etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		db:            etc.Get(defaultDBKey, defaultDB).Int(),
		username:      etc.Get(defaultUsernameKey).String(),
		password:      etc.Get(defaultPasswordKey).String(),
		certFile:      etc.Get(defaultCertFileKey).String(),
		keyFile:       etc.Get(defaultKeyFileKey).String(),
		caFile:        etc.Get(defaultCAFileKey).String(),
		maxRetries:    etc.Get(defaultMaxRetriesKey, defaultMaxRetries).Int(),
		prefix:        etc.Get(defaultPrefixKey, defaultPrefix).String(),
		nilValue:      etc.Get(defaultNilValueKey, defaultNilValue).String(),
		nilExpiration: etc.Get(defaultNilExpirationKey, defaultNilExpiration).Duration(),
		minExpiration: etc.Get(defaultMinExpirationKey, defaultMinExpiration).Duration(),
		maxExpiration: etc.Get(defaultMaxExpirationKey, defaultMaxExpiration).Duration(),
	}
}

// WithAddrs 设置连接地址
// @param addrs ...string 一个或多个 Redis 节点地址
// @return @1 Option 配置项
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDB 设置数据库号
// @param db int 数据库编号
// @return @1 Option 配置项
func WithDB(db int) Option {
	return func(o *options) { o.db = db }
}

// WithUsername 设置用户名
// @param username string 认证用户名
// @return @1 Option 配置项
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword 设置密码
// @param password string 认证密码
// @return @1 Option 配置项
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithCredentials 设置证书、密钥、CA证书
// @param certFile string 客户端证书文件路径
// @param keyFile string 客户端私钥文件路径
// @param caFile string CA 证书文件路径
// @return @1 Option 配置项
func WithCredentials(certFile, keyFile, caFile string) Option {
	return func(o *options) { o.certFile, o.keyFile, o.caFile = certFile, keyFile, caFile }
}

// WithMaxRetries 设置最大重试次数
// @param maxRetries int 最大重试次数
// @return @1 Option 配置项
func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

// WithClient 设置外部客户端
// @param client redis.UniversalClient 外部客户端实例
// @return @1 Option 配置项
func WithClient(client redis.UniversalClient) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix 设置前缀
// @param prefix string key 前缀
// @return @1 Option 配置项
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithNilValue 设置空值
// @param nilValue string 空值占位字符串
// @return @1 Option 配置项
func WithNilValue(nilValue string) Option {
	return func(o *options) { o.nilValue = nilValue }
}

// WithNilExpiration 设置空值过期时间
// @param nilExpiration time.Duration 空值过期时间
// @return @1 Option 配置项
func WithNilExpiration(nilExpiration time.Duration) Option {
	return func(o *options) {
		if nilExpiration > 0 {
			o.nilExpiration = nilExpiration
		}
	}
}

// WithMinExpiration 设置最小过期时间
// @param minExpiration time.Duration 最小过期时间
// @return @1 Option 配置项
func WithMinExpiration(minExpiration time.Duration) Option {
	return func(o *options) {
		if minExpiration > 0 {
			o.minExpiration = minExpiration
		}
	}
}

// WithMaxExpiration 设置最大过期时间
// @param maxExpiration time.Duration 最大过期时间
// @return @1 Option 配置项
func WithMaxExpiration(maxExpiration time.Duration) Option {
	return func(o *options) {
		if maxExpiration > 0 {
			o.maxExpiration = maxExpiration
		}
	}
}
