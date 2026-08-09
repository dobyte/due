package redis

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
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
		}

		c.opts.client, c.builtin = redis.NewUniversalClient(options), true
	}

	return c
}

// Has 检测缓存是否存在
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	if c.err != nil {
		return false, c.err
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
func (c *Cache) Get(ctx context.Context, key string, def ...any) cache.Result {
	if c.err != nil {
		return cache.NewResult(nil, c.err)
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
func (c *Cache) Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	if c.err != nil {
		return c.err
	}

	if len(expiration) > 0 {
		return c.opts.client.Set(ctx, c.AddPrefix(key), xconv.String(value), expiration[0]).Err()
	} else {
		expiration := time.Duration(xrand.Int64(int64(c.opts.minExpiration), int64(c.opts.maxExpiration)))

		return c.opts.client.Set(ctx, c.AddPrefix(key), xconv.String(value), expiration).Err()
	}
}

// GetSet 获取设置缓存值
func (c *Cache) GetSet(ctx context.Context, key string, fn cache.SetValueFunc) cache.Result {
	if c.err != nil {
		return cache.NewResult(nil, c.err)
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

		expiration := time.Duration(xrand.Int64(int64(c.opts.minExpiration), int64(c.opts.maxExpiration)))

		if err = c.opts.client.Set(ctx, key, xconv.String(val), expiration).Err(); err != nil {
			return cache.NewResult(nil, err), nil
		} else {
			return cache.NewResult(val, nil), nil
		}
	})

	return rst.(cache.Result)
}

// Delete 删除缓存
func (c *Cache) Delete(ctx context.Context, keys ...string) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}

	if len(keys) == 0 {
		return 0, nil
	}

	allKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		allKeys = append(allKeys, c.AddPrefix(key))
	}

	return c.opts.client.Del(ctx, allKeys...).Result()
}

// IncrInt 整数自增
func (c *Cache) IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}

	return c.opts.client.IncrBy(ctx, c.AddPrefix(key), value).Result()
}

// IncrFloat 浮点数自增
func (c *Cache) IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if c.err != nil {
		return 0, c.err
	}

	return c.opts.client.IncrByFloat(ctx, c.AddPrefix(key), value).Result()
}

// DecrInt 整数自减
func (c *Cache) DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}

	return c.opts.client.DecrBy(ctx, c.AddPrefix(key), value).Result()
}

// DecrFloat 浮点数自减
func (c *Cache) DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if c.err != nil {
		return 0, c.err
	}

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
	if c.err != nil {
		return nil
	}

	return c.opts.client
}

// Close 关闭缓存
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
