package redis

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddr              = "127.0.0.1:6379" // 默认客户端连接地址
	defaultDB                = 0                // 默认数据库号
	defaultMaxRetries        = 3                // 默认最大重试次数
	defaultPrefix            = "due:lock"       // 默认key前缀
	defaultExpiration        = "3s"             // 默认锁过期时间
	defaultAcquireInterval   = "20ms"           // 默认循环获取锁的时间间隔
	defaultAcquireMaxRetries = 0                // 默认循环获取锁的最大重试次数，0表示无限次重试
)

const (
	defaultAddrsKey             = "etc.lock.redis.addrs"             // 连接地址配置key
	defaultDBKey                = "etc.lock.redis.db"                // 数据库号配置key
	defaultMaxRetriesKey        = "etc.lock.redis.maxRetries"        // 最大重试次数配置key
	defaultPrefixKey            = "etc.lock.redis.prefix"            // key前缀配置key
	defaultUsernameKey          = "etc.lock.redis.username"          // 用户名配置key
	defaultPasswordKey          = "etc.lock.redis.password"          // 密码配置key
	defaultCertFileKey          = "etc.lock.redis.certFile"          // 客户端证书配置key
	defaultKeyFileKey           = "etc.lock.redis.keyFile"           // 客户端密钥配置key
	defaultCaFileKey            = "etc.lock.redis.caFile"            // CA证书配置key
	defaultExpirationKey        = "etc.lock.redis.expiration"        // 锁过期时间配置key
	defaultAcquireIntervalKey   = "etc.lock.redis.acquireInterval"   // 循环获取锁间隔配置key
	defaultAcquireMaxRetriesKey = "etc.lock.redis.acquireMaxRetries" // 循环获取锁最大重试次数配置key
)

// Option 锁配置函数
type Option func(o *options)

// 锁配置项
// 控制 redis 客户端、key前缀、锁过期时间以及循环获取锁的行为；
// 各参数均优先从配置环境读取，未配置时采用默认值
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
	// 内建客户端 TLS 配置，默认为空
	certFile string

	// 客户端密钥
	// 内建客户端 TLS 配置，默认为空
	keyFile string

	// CA证书
	// 内建客户端 TLS 配置，默认为空
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
	// 注意：redis 过期时间精度为毫秒，NewMaker 会将小于1毫秒的配置收敛为1毫秒
	expiration time.Duration

	// 循环获取锁的频率间隔时间，默认为20ms
	acquireInterval time.Duration

	// 循环获取锁的最大重试次数，默认为无限次
	acquireMaxRetries int
}

// 创建默认锁配置项
// 依次从配置环境读取各参数并填充默认值，未配置时采用内置默认值
// @return @1 *options 默认锁配置项
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

// WithAddrs 设置客户端连接地址
// 设置内建客户端的连接地址，未指定外部客户端时生效
// @param addrs ...string 客户端连接地址
// @return @1 Option 锁配置函数
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDB 设置数据库号
// 设置内建客户端使用的数据库号，未指定外部客户端时生效
// @param db int 数据库号
// @return @1 Option 锁配置函数
func WithDB(db int) Option {
	return func(o *options) { o.db = db }
}

// WithUsername 设置用户名
// 设置内建客户端的认证用户名，未指定外部客户端时生效
// @param username string 用户名
// @return @1 Option 锁配置函数
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword 设置密码
// 设置内建客户端的认证密码，未指定外部客户端时生效
// @param password string 密码
// @return @1 Option 锁配置函数
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithCredentials 设置证书、密钥、CA证书
// 设置内建客户端的 TLS 双向认证配置，未指定外部客户端时生效；
// 三个参数均非空时，NewMaker 才会构建 TLS 配置
// @param certFile string 客户端证书文件路径
// @param keyFile string 客户端密钥文件路径
// @param caFile string CA证书文件路径
// @return @1 Option 锁配置函数
func WithCredentials(certFile, keyFile, caFile string) Option {
	return func(o *options) { o.certFile, o.keyFile, o.caFile = certFile, keyFile, caFile }
}

// WithMaxRetries 设置最大重试次数
// 设置内建客户端在网络请求失败时的最大重试次数，未指定外部客户端时生效
// @param maxRetries int 最大重试次数
// @return @1 Option 锁配置函数
func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

// WithClient 设置外部客户端
// 设置外部客户端，存在外部客户端时优先使用之；此时构建器关闭将不再管理客户端生命周期
// @param client redis.UniversalClient 外部客户端
// @return @1 Option 锁配置函数
func WithClient(client redis.UniversalClient) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix 设置前缀
// 设置key前缀，最终锁key为 prefix + ":" + name；前缀为空时不进行拼接
// @param prefix string key前缀
// @return @1 Option 锁配置函数
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithExpiration 设置锁过期时间
// 锁的过期时长，同时是后台续租刷新的目标时长；redis 精度为毫秒，小于1毫秒的配置会被收敛为1毫秒；
// 注意：后台续租间隔为过期时间的一半，过短的过期时间(如几十毫秒级)会使续租难以在锁过期前完成刷新，
// 需持续持有锁的续租场景建议不小于1s
// @param expiration time.Duration 锁过期时间
// @return @1 Option 锁配置函数
func WithExpiration(expiration time.Duration) Option {
	return func(o *options) { o.expiration = expiration }
}

// WithAcquireInterval 设置获取锁的时间间隔
// 循环获取锁失败后再次尝试的时间间隔；小于等于0时在 NewMaker 中收敛为默认值20ms
// @param acquireInterval time.Duration 获取锁的时间间隔
// @return @1 Option 锁配置函数
func WithAcquireInterval(acquireInterval time.Duration) Option {
	return func(o *options) { o.acquireInterval = acquireInterval }
}

// WithAcquireMaxRetries 设置循环获取锁的最大重试次数
// 达到最大重试次数后仍未获取成功则返回超时错误；0表示无限次重试直至成功或上下文取消
// @param acquireMaxRetries int 最大重试次数
// @return @1 Option 锁配置函数
func WithAcquireMaxRetries(acquireMaxRetries int) Option {
	return func(o *options) { o.acquireMaxRetries = acquireMaxRetries }
}
