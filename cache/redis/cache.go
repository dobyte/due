package redis

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xrand"
	"github.com/dobyte/due/v2/utils/xreflect"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Cache struct {
	err     error
	opts    *options
	builtin bool
	closed  atomic.Bool
	sfg     singleflight.Group
}

// NewCache 创建一个 Redis 缓存实例
// @param opts ...Option 可选配置项，用于覆盖默认配置
// @return @1 *Cache 缓存实例
func NewCache(opts ...Option) *Cache {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &Cache{}
	c.opts = o

	if c.opts.client == nil {
		options := &redis.UniversalOptions{
			Addrs:      c.opts.addrs,
			DB:         c.opts.db,
			Username:   c.opts.username,
			Password:   c.opts.password,
			MaxRetries: c.opts.maxRetries,
		}

		if c.opts.certFile != "" && c.opts.keyFile != "" && c.opts.caFile != "" {
			if options.TLSConfig, c.err = tls.MakeRedisTLSConfig(c.opts.certFile, c.opts.keyFile, c.opts.caFile); c.err != nil {
				return c
			}
		} else {
			if c.opts.certFile != "" || c.opts.keyFile != "" || c.opts.caFile != "" {
				log.Warn("redis cache: certFile or keyFile or caFile is empty")
			}
		}

		c.opts.client, c.builtin = redis.NewUniversalClient(options), true
	}

	return c
}

// Has 检测缓存是否存在
// @param ctx context.Context 上下文
// @param key string 缓存键
// @return @1 bool 缓存是否存在
// @return @2 error 错误信息
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	if err := c.check(); err != nil {
		return false, err
	}

	key = c.AddPrefix(key)

	val, err := c.opts.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		} else {
			return false, err
		}
	}

	return val != c.opts.nilValue, nil
}

// Get 获取缓存值
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param def ...any 可选默认值，当缓存不存在时返回该默认值
// @return @1 cache.Result 缓存结果
func (c *Cache) Get(ctx context.Context, key string, def ...any) cache.Result {
	if err := c.check(); err != nil {
		return cache.NewResult(nil, err)
	}

	key = c.AddPrefix(key)

	val, err := c.opts.client.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return cache.NewResult(nil, err)
	}

	if errors.Is(err, redis.Nil) || val == c.opts.nilValue {
		if len(def) > 0 {
			return cache.NewResult(def[0])
		} else {
			return cache.NewResult(nil, errors.ErrNil)
		}
	}

	return cache.NewResult(val)
}

// Set 设置缓存值
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value any 缓存值
// @param expiration ...time.Duration 过期时间，省略表示使用过期时间范围随机，>0表示使用具体的过期时间，=-1表示保持原有过期时间，<-1表示永不过期
// @return @1 error 错误信息
func (c *Cache) Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	if err := c.check(); err != nil {
		return err
	}

	var ttl time.Duration

	if len(expiration) > 0 {
		ttl = expiration[0]
	} else {
		ttl = xrand.Duration(c.opts.minExpiration, c.opts.maxExpiration)
	}

	return c.opts.client.Set(ctx, c.AddPrefix(key), xconv.String(value), ttl).Err()
}

// GetSet 获取缓存值，若不存在则通过 fn 生成后写入并返回
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param fn cache.SetValueFunc 缓存未命中时执行的回调，用于生成缓存值
// @return @1 cache.Result 缓存结果
func (c *Cache) GetSet(ctx context.Context, key string, fn cache.SetValueFunc) cache.Result {
	if err := c.check(); err != nil {
		return cache.NewResult(nil, err)
	}

	key = c.AddPrefix(key)

	if val, err := c.opts.client.Get(ctx, key).Result(); err == nil {
		if val == c.opts.nilValue {
			return cache.NewResult(nil, errors.ErrNil)
		} else {
			return cache.NewResult(val)
		}
	} else if !errors.Is(err, redis.Nil) {
		return cache.NewResult(nil, err)
	}

	rst, _, _ := c.sfg.Do(key+":set", func() (any, error) {
		if val, err := c.opts.client.Get(ctx, key).Result(); err == nil {
			if val == c.opts.nilValue {
				return cache.NewResult(nil, errors.ErrNil), nil
			} else {
				return cache.NewResult(val), nil
			}
		} else if !errors.Is(err, redis.Nil) {
			return cache.NewResult(nil, err), nil
		}

		val, err := fn()
		if err != nil {
			return cache.NewResult(nil, err), nil
		}

		if val == nil || xreflect.IsNil(val) {
			if err = c.opts.client.Set(ctx, key, c.opts.nilValue, c.opts.nilExpiration).Err(); err != nil {
				return cache.NewResult(nil, err), nil
			} else {
				return cache.NewResult(nil, errors.ErrNil), nil
			}
		}

		ttl := xrand.Duration(c.opts.minExpiration, c.opts.maxExpiration)

		if err = c.opts.client.Set(ctx, key, xconv.String(val), ttl).Err(); err != nil {
			return cache.NewResult(nil, err), nil
		} else {
			return cache.NewResult(val, nil), nil
		}
	})

	return rst.(cache.Result)
}

// Delete 删除缓存
// @param ctx context.Context 上下文
// @param keys ...string 缓存键，可传入多个
// @return @1 int64 实际删除的 key 数量
// @return @2 error 错误信息
func (c *Cache) Delete(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	if err := c.check(); err != nil {
		return 0, err
	}

	allKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		allKeys = append(allKeys, c.AddPrefix(key))
	}

	return c.opts.client.Del(ctx, allKeys...).Result()
}

// IncrInt 整数自增
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value int64 自增步长
// @return @1 int64 自增后的值
// @return @2 error 错误信息
func (c *Cache) IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.IncrBy(ctx, c.AddPrefix(key), value).Result()
}

// IncrFloat 浮点数自增
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value float64 自增步长
// @return @1 float64 自增后的值
// @return @2 error 错误信息
func (c *Cache) IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.IncrByFloat(ctx, c.AddPrefix(key), value).Result()
}

// DecrInt 整数自减
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value int64 自减步长
// @return @1 int64 自减后的值
// @return @2 error 错误信息
func (c *Cache) DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.DecrBy(ctx, c.AddPrefix(key), value).Result()
}

// DecrFloat 浮点数自减
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value float64 自减步长
// @return @1 float64 自减后的值
// @return @2 error 错误信息
func (c *Cache) DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.IncrByFloat(ctx, c.AddPrefix(key), -value).Result()
}

// AddPrefix 添加Key前缀
// @param key string 缓存键
// @return @1 string 添加前缀后的完整键名
func (c *Cache) AddPrefix(key string) string {
	if c.opts.prefix == "" {
		return key
	} else {
		return c.opts.prefix + ":" + key
	}
}

// Client 获取客户端
// @return @1 any 底层 Redis 客户端
func (c *Cache) Client() any {
	if err := c.check(); err != nil {
		return nil
	}

	return c.opts.client
}

// check 检查缓存是否已关闭
func (c *Cache) check() error {
	if c.err != nil {
		return c.err
	}

	if c.closed.Load() {
		return errors.ErrCacheClosed
	}

	return nil
}

// Close 关闭缓存
// @return @1 error 错误信息
func (c *Cache) Close() error {
	if c.err != nil {
		return c.err
	}

	if c.closed.Swap(true) {
		return errors.ErrCacheClosed
	}

	if c.builtin && c.opts.client != nil {
		return c.opts.client.Close()
	}

	return nil
}
