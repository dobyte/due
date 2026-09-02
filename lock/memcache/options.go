package memcache

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultAddr              = "127.0.0.1:11211" // 默认客户端连接地址
	defaultPrefix            = "due:lock"        // 默认key前缀
	defaultExpiration        = "3s"              // 默认锁过期时间
	defaultAcquireInterval   = "20ms"            // 默认循环获取锁的时间间隔
	defaultAcquireMaxRetries = 0                 // 默认循环获取锁的最大重试次数，0表示无限次重试
)

const (
	defaultAddrsKey             = "etc.lock.memcache.addrs"             // 连接地址配置key
	defaultPrefixKey            = "etc.lock.memcache.prefix"            // key前缀配置key
	defaultExpirationKey        = "etc.lock.memcache.expiration"        // 锁过期时间配置key
	defaultAcquireIntervalKey   = "etc.lock.memcache.acquireInterval"   // 循环获取锁间隔配置key
	defaultAcquireMaxRetriesKey = "etc.lock.memcache.acquireMaxRetries" // 循环获取锁最大重试次数配置key
)

// Option 锁配置函数
type Option func(o *options)

// 锁配置项
// 控制 memcached 客户端、key前缀、锁过期时间以及循环获取锁的行为；
// 各参数均优先从配置环境读取，未配置时采用默认值
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
	// 注意：memcached 过期精度为秒且 0 表示永不过期，NewMaker 会将小于 1s 的配置收敛为 1s；
	// memcached 按整秒记录过期时间，实际存活时长可能比配置值短近1秒；Acquire/TryAcquire(未指定固定过期)
	// 会自动续租，续租间隔为过期时间的一半，过期时间小于等于2s时续租难以在锁过期前完成刷新，建议不小于3s
	expiration time.Duration

	// 循环获取锁的频率间隔时间，默认为20ms
	// 注意：小于等于0时在 NewMaker 中收敛为默认值20ms，避免重试退化为无退避忙等循环
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
		prefix:            etc.Get(defaultPrefixKey, defaultPrefix).String(),
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

// WithClient 设置外部客户端
// 设置外部客户端，存在外部客户端时优先使用之；此时构建器关闭将不再管理客户端生命周期
// @param client *memcache.Client 外部客户端
// @return @1 Option 锁配置函数
func WithClient(client *memcache.Client) Option {
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
// 锁的过期时长，同时是后台续租刷新的目标时长；memcached 精度为秒，小于1秒的配置会被收敛为1秒；
// 注意：memcached 按整秒记录过期时间，实际存活时长可能比配置值短近1秒；后台续租间隔为过期时间的一半，
// 小于2秒的过期时间可能使续租难以在锁过期前完成刷新，需持续持有锁的续租场景建议不小于3s
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
