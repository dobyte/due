package redis

import (
	"context"
	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xrand"
	"github.com/dobyte/due/v2/utils/xreflect"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
	"time"
)

type Cache struct {
	opts *options
	sfg  singleflight.Group
}

func NewCache(opts ...Option) *Cache {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.client == nil {
		o.client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		})
	}

	c := &Cache{}
	c.opts = o

	return c
}

// Has 检测缓存是否存在
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (interface{}, error) {
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
func (c *Cache) Get(ctx context.Context, key string, def ...interface{}) cache.Result {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (interface{}, error) {
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
func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration ...time.Duration) error {
	if len(expiration) > 0 {
		return c.opts.client.Set(ctx, c.AddPrefix(key), xconv.String(value), expiration[0]).Err()
	} else {
		return c.opts.client.Set(ctx, c.AddPrefix(key), xconv.String(value), redis.KeepTTL).Err()
	}
}

// GetSet 获取设置缓存值
func (c *Cache) GetSet(ctx context.Context, key string, fn cache.SetValueFunc) cache.Result {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (interface{}, error) {
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

	rst, _, _ := c.sfg.Do(key+":set", func() (interface{}, error) {
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
func (c *Cache) Delete(ctx context.Context, keys ...string) (bool, error) {
	prefixedKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if key != "" {
			prefixedKeys = append(prefixedKeys, c.AddPrefix(key))
		}
	}

	if len(prefixedKeys) == 0 {
		return false, nil
	}

	num, err := c.opts.client.Del(ctx, prefixedKeys...).Result()
	return num > 1, err
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
func (c *Cache) Client() interface{} {
	return c.opts.client
}
