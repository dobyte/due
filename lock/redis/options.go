package redis

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr              = "127.0.0.1:6379"
	defaultDB                = 0
	defaultMaxRetries        = 3
	defaultPrefix            = "due:lock"
	defaultExpiration        = "3s"
	defaultAcquireInterval   = "20ms"
	defaultAcquireMaxRetries = 0
)

const (
	defaultAddrsKey             = "etc.lock.redis.addrs"
	defaultDBKey                = "etc.lock.redis.db"
	defaultMaxRetriesKey        = "etc.lock.redis.maxRetries"
	defaultPrefixKey            = "etc.lock.redis.prefix"
	defaultUsernameKey          = "etc.lock.redis.username"
	defaultPasswordKey          = "etc.lock.redis.password"
	defaultCertFileKey          = "etc.lock.redis.certFile"
	defaultKeyFileKey           = "etc.lock.redis.keyFile"
	defaultCaFileKey            = "etc.lock.redis.caFile"
	defaultExpirationKey        = "etc.lock.redis.expiration"
	defaultAcquireIntervalKey   = "etc.lock.redis.acquireInterval"
	defaultAcquireMaxRetriesKey = "etc.lock.redis.acquireMaxRetries"
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
		db:                etc.Get(defaultDBKey, defaultDB).Int(),
		maxRetries:        etc.Get(defaultMaxRetriesKey, defaultMaxRetries).Int(),
		prefix:            etc.Get(defaultPrefixKey, defaultPrefix).String(),
		username:          etc.Get(defaultUsernameKey).String(),
		password:          etc.Get(defaultPasswordKey).String(),
		certFile:          etc.Get(defaultCertFileKey).String(),
		keyFile:           etc.Get(defaultKeyFileKey).String(),
		caFile:            etc.Get(defaultCaFileKey).String(),
		expiration:        etc.Get(defaultExpirationKey, defaultExpiration).Duration(),
		acquireInterval:   etc.Get(defaultAcquireIntervalKey, defaultAcquireInterval).Duration(),
		acquireMaxRetries: etc.Get(defaultAcquireMaxRetriesKey, defaultAcquireMaxRetries).Int(),
	}
}

// WithAddrs 设置连接地址
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDB 设置数据库号
func WithDB(db int) Option {
	return func(o *options) { o.db = db }
}

// WithUsername 设置用户名
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword 设置密码
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithCredentials 设置证书、密钥、CA证书
func WithCredentials(certFile, keyFile, caFile string) Option {
	return func(o *options) { o.certFile, o.keyFile, o.caFile = certFile, keyFile, caFile }
}

// WithMaxRetries 设置最大重试次数
func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

// WithClient 设置外部客户端
func WithClient(client redis.UniversalClient) Option {
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
