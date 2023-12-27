package redis

import (
	"context"
	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
	"time"
)

type Cache struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   *options
	sfg    singleflight.Group
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
	c.ctx, c.cancel = context.WithCancel(o.ctx)
	c.opts = o

	return c
}

// AddPrefix 添加Key前缀
func (c *Cache) AddPrefix(key string) string {
	if c.opts.prefix == "" {
		return key
	} else {
		return c.opts.prefix + ":" + key
	}
}

// Has 检测缓存是否存在
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (interface{}, error) {
		return c.opts.client.Get(ctx, key).Result()
	})
	if err != nil {
		if err == redis.Nil {
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
	if err != nil {
		if err != redis.Nil {
			return cache.NewResult(nil, err)
		}

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
