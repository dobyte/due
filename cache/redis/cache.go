package redis

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xrand"
	"github.com/dobyte/due/v2/utils/xreflect"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
)

var _ cache.Cache = (*Cache)(nil)

type Cache struct {
	err     error
	opts    *options
	builtin bool
	sfg     singleflight.Group
}

func NewCache(opts ...Option) *Cache {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &Cache{}

	defer func() {
		if c.err == nil {
			c.opts = o
		}
	}()

	if o.client == nil {
		options := &redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		}

		if o.certFile != "" && o.keyFile != "" && o.caFile != "" {
			if options.TLSConfig, c.err = tls.MakeRedisTLSConfig(o.certFile, o.keyFile, o.caFile); c.err != nil {
				return c
			}
		}

		o.client, c.builtin = redis.NewUniversalClient(options), true
	}

	return c
}

// Has 检测缓存是否存在
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (any, error) {
		return c.opts.client.Get(ctx, key).Result()
	})
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}

	if val.(string) == c.opts.nilValue {
		return false, nil
	}

	return true, nil
}

// Get 获取缓存值
func (c *Cache) Get(ctx context.Context, key string, def ...any) cache.Result {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (any, error) {
		return c.opts.client.Get(ctx, key).Result()
	})
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
func (c *Cache) Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	if len(expiration) > 0 {
		return c.opts.client.Set(ctx, c.AddPrefix(key), xconv.String(value), expiration[0]).Err()
	} else {
		return c.opts.client.Set(ctx, c.AddPrefix(key), xconv.String(value), redis.KeepTTL).Err()
	}
}

// GetSet 获取设置缓存值
func (c *Cache) GetSet(ctx context.Context, key string, fn cache.SetValueFunc) cache.Result {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (any, error) {
		return c.opts.client.Get(ctx, key).Result()
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return cache.NewResult(nil, err)
	}

	if err == nil {
		if val == c.opts.nilValue {
			return cache.NewResult(nil, errors.ErrNil)
		} else {
			return cache.NewResult(val)
		}
	}

	rst, _, _ := c.sfg.Do(key+":set", func() (any, error) {
		val, err := fn()
		if err != nil {
			return cache.NewResult(nil, err), nil
		}

		if val == nil || xreflect.IsNil(val) {
			if err = c.opts.client.Set(ctx, key, c.opts.nilValue, c.opts.nilExpiration).Err(); err != nil {
				return cache.NewResult(nil, err), nil
			}
			return cache.NewResult(nil, errors.ErrNil), nil
		}

		expiration := time.Duration(xrand.Int64(int64(c.opts.minExpiration), int64(c.opts.maxExpiration)))

		if err = c.opts.client.Set(ctx, key, xconv.String(val), expiration).Err(); err != nil {
			return cache.NewResult(nil, err), nil
		}

		return cache.NewResult(val, nil), nil
	})

	return rst.(cache.Result)
}

// Delete 删除缓存
func (c *Cache) Delete(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	for i, key := range keys {
		keys[i] = c.AddPrefix(key)
	}

	return c.opts.client.Del(ctx, keys...).Result()
}

// IncrInt 整数自增
func (c *Cache) IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	return c.opts.client.IncrBy(ctx, c.AddPrefix(key), value).Result()
}

// IncrFloat 浮点数自增
func (c *Cache) IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	return c.opts.client.IncrByFloat(ctx, c.AddPrefix(key), value).Result()
}

// DecrInt 整数自减
func (c *Cache) DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	return c.opts.client.DecrBy(ctx, c.AddPrefix(key), value).Result()
}

// DecrFloat 浮点数自减
func (c *Cache) DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	return c.opts.client.IncrByFloat(ctx, c.AddPrefix(key), -value).Result()
}

// AddPrefix 添加Key前缀
func (c *Cache) AddPrefix(key string) string {
	if c.opts.prefix == "" {
		return key
	} else {
		return c.opts.prefix + ":" + key
	}
}

// Client 获取客户端
func (c *Cache) Client() any {
	return c.opts.client
}

// Close 关闭缓存
func (c *Cache) Close() error {
	if c.builtin {
		return c.opts.client.Close()
	}

	return nil
}

// Ping 检查缓存连接
func (c *Cache) Ping(ctx context.Context) error {
	return c.opts.client.Ping(ctx).Err()
}
