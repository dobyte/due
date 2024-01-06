package memcache

import (
	"context"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"golang.org/x/sync/singleflight"
	"reflect"
	"time"
)

type Cache struct {
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
	c.opts = o

	if o.client == nil {
		c.builtin = true
		o.client = memcache.New(o.addrs...)
	}

	return c
}

// Has 检测缓存是否存在
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (interface{}, error) {
		item, err := c.opts.client.Get(key)
		if err != nil {
			return nil, err
		}

		return xconv.String(item.Value), nil
	})
	if err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
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
		item, err := c.opts.client.Get(key)
		if err != nil {
			return nil, err
		}

		return xconv.String(item.Value), nil
	})
	if err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
		return cache.NewResult(nil, err)
	}

	if errors.Is(err, memcache.ErrCacheMiss) || val.(string) == c.opts.nilValue {
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
	if len(expiration) > 0 && expiration[0] > 0 {
		return c.opts.client.Set(&memcache.Item{
			Key:        c.AddPrefix(key),
			Value:      xconv.Bytes(value),
			Expiration: int32(expiration[0] / time.Second),
		})
	} else {
		return c.opts.client.Set(&memcache.Item{
			Key:   c.AddPrefix(key),
			Value: xconv.Bytes(value),
		})
	}
}

// GetSet 获取设置缓存值
func (c *Cache) GetSet(ctx context.Context, key string, fn cache.SetValueFunc) cache.Result {
	key = c.AddPrefix(key)

	val, err, _ := c.sfg.Do(key, func() (interface{}, error) {
		item, err := c.opts.client.Get(key)
		if err != nil {
			return nil, err
		}

		return xconv.String(item.Value), nil
	})
	if err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
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
		val, expiration, err := fn()
		if err != nil {
			return cache.NewResult(nil, err), nil
		}

		if val == nil || reflect.ValueOf(val).IsNil() {
			if err = c.opts.client.Set(&memcache.Item{
				Key:        key,
				Value:      xconv.Bytes(c.opts.nilValue),
				Expiration: int32(c.opts.nilExpiration / time.Second),
			}); err != nil {
				return cache.NewResult(nil, err), nil
			}
			return cache.NewResult(nil, errors.ErrNil), nil
		}

		if err = c.opts.client.Set(&memcache.Item{
			Key:        key,
			Value:      xconv.Bytes(val),
			Expiration: int32(expiration / time.Second),
		}); err != nil {
			return cache.NewResult(nil, err), nil
		}

		return cache.NewResult(val, nil), nil
	})

	return rst.(cache.Result)
}

// Delete 删除缓存
func (c *Cache) Delete(ctx context.Context, key string) (bool, error) {
	err := c.opts.client.Delete(c.AddPrefix(key))
	if err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// IncrInt 整数自增
func (c *Cache) IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	if value < 0 {
		return c.DecrInt(ctx, key, 0-value)
	}

	key = c.AddPrefix(key)

	newValue, err := c.opts.client.Increment(key, uint64(value))
	if err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			err = c.opts.client.Add(&memcache.Item{
				Key:   key,
				Value: xconv.Bytes(xconv.String(value)),
			})
			if err != nil {
				if errors.Is(err, memcache.ErrNotStored) {
					newValue, err = c.opts.client.Increment(key, uint64(value))
					if err != nil {
						return 0, err
					}

					return int64(newValue), nil
				}
				return 0, err
			}

			return value, nil
		} else {
			return 0, err
		}
	}

	return int64(newValue), nil
}

// IncrFloat 浮点数自增
func (c *Cache) IncrFloat(ctx context.Context, key string, value float64) (float64, error) {

}

// DecrInt 整数自减
func (c *Cache) DecrInt(ctx context.Context, key string, value int64) (int64, error) {

}

// DecrFloat 浮点数自减
func (c *Cache) DecrFloat(ctx context.Context, key string, value float64) (float64, error) {

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

// Close 关闭客户端
func (c *Cache) Close() error {
	if !c.builtin {
		return nil
	}

	return c.opts.client.Close()
}
