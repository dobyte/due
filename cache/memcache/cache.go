package memcache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xrand"
	"github.com/dobyte/due/v2/utils/xreflect"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type Cache struct {
	opts    *options
	builtin bool
	closed  atomic.Bool
	sfg     singleflight.Group
}

// NewCache 创建一个 Memcache 缓存实例
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
		c.opts.client, c.builtin = memcache.New(c.opts.addrs...), true
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

	if item, err := c.opts.client.Get(c.AddPrefix(key)); err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			return false, nil
		} else {
			return false, err
		}
	} else {
		return xconv.String(item.Value) != c.opts.nilValue, nil
	}
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

	item, err := c.opts.client.Get(c.AddPrefix(key))
	if err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
		return cache.NewResult(nil, err)
	}

	if errors.Is(err, memcache.ErrCacheMiss) || xconv.String(item.Value) == c.opts.nilValue {
		if len(def) > 0 {
			return cache.NewResult(def[0])
		} else {
			return cache.NewResult(nil, errors.ErrNil)
		}
	}

	return cache.NewResult(xconv.String(item.Value))
}

// Set 设置缓存值
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value any 缓存值
// @param expiration ...time.Duration 可选过期时间，省略时在最小/最大过期时间区间内随机生成
// @return @1 error 错误信息
func (c *Cache) Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	if err := c.check(); err != nil {
		return err
	}

	var ttl int32

	if len(expiration) > 0 {
		ttl = int32(expiration[0].Seconds())
	} else {
		ttl = int32(xrand.Duration(c.opts.minExpiration, c.opts.maxExpiration).Seconds())
	}

	return c.opts.client.Set(&memcache.Item{
		Key:        c.AddPrefix(key),
		Value:      []byte(xconv.String(value)),
		Expiration: ttl,
	})
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

	if item, err := c.opts.client.Get(key); err == nil {
		val := xconv.String(item.Value)
		if val == c.opts.nilValue {
			return cache.NewResult(nil, errors.ErrNil)
		} else {
			return cache.NewResult(val)
		}
	} else if !errors.Is(err, memcache.ErrCacheMiss) {
		return cache.NewResult(nil, err)
	}

	rst, _, _ := c.sfg.Do(key+":set", func() (any, error) {
		if item, err := c.opts.client.Get(key); err == nil {
			val := xconv.String(item.Value)
			if val == c.opts.nilValue {
				return cache.NewResult(nil, errors.ErrNil), nil
			} else {
				return cache.NewResult(val), nil
			}
		} else if !errors.Is(err, memcache.ErrCacheMiss) {
			return cache.NewResult(nil, err), nil
		}

		val, err := fn()
		if err != nil {
			return cache.NewResult(nil, err), nil
		}

		if val == nil || xreflect.IsNil(val) {
			if err = c.opts.client.Set(&memcache.Item{
				Key:        key,
				Value:      xconv.Bytes(c.opts.nilValue),
				Expiration: int32(c.opts.nilExpiration.Seconds()),
			}); err != nil {
				return cache.NewResult(nil, err), nil
			} else {
				return cache.NewResult(nil, errors.ErrNil), nil
			}
		}

		ttl := int32(xrand.Duration(c.opts.minExpiration, c.opts.maxExpiration).Seconds())

		if err = c.opts.client.Set(&memcache.Item{
			Key:        key,
			Value:      xconv.Bytes(val),
			Expiration: ttl,
		}); err != nil {
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

	total := int64(0)
	eg, _ := errgroup.WithContext(ctx)

	for _, key := range keys {
		key = c.AddPrefix(key)

		eg.Go(func() error {
			if err := c.opts.client.Delete(key); err != nil {
				if errors.Is(err, memcache.ErrCacheMiss) {
					return nil
				}

				return err
			}

			atomic.AddInt64(&total, 1)

			return nil
		})
	}

	err := eg.Wait()

	if total > 0 {
		return total, nil
	}

	return 0, err
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

	if value < 0 {
		return c.DecrInt(ctx, key, 0-value)
	}

	key = c.AddPrefix(key)

	if newValue, err := c.opts.client.Increment(key, uint64(value)); err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			if err = c.opts.client.Add(&memcache.Item{
				Key:   key,
				Value: xconv.Bytes(xconv.String(value)),
			}); err != nil {
				if errors.Is(err, memcache.ErrNotStored) {
					if newValue, err = c.opts.client.Increment(key, uint64(value)); err != nil {
						return 0, err
					} else {
						return int64(newValue), nil
					}
				} else {
					return 0, err
				}
			}

			return value, nil
		} else {
			return 0, err
		}
	} else {
		return int64(newValue), nil
	}
}

// IncrFloat 浮点数自增，鉴于memcache不支持浮点数，这里通过整数自增来实现
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value float64 自增步长
// @return @1 float64 自增后的值
// @return @2 error 错误信息
func (c *Cache) IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	if newValue, err := c.IncrInt(ctx, key, int64(value)); err != nil {
		return 0, err
	} else {
		return float64(newValue), nil
	}
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

	if value < 0 {
		return c.IncrInt(ctx, key, 0-value)
	}

	key = c.AddPrefix(key)

	if newValue, err := c.opts.client.Decrement(key, uint64(value)); err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			if err = c.opts.client.Add(&memcache.Item{
				Key:   key,
				Value: xconv.Bytes(xconv.String(value)),
			}); err != nil {
				if errors.Is(err, memcache.ErrNotStored) {
					if newValue, err = c.opts.client.Decrement(key, uint64(value)); err != nil {
						return 0, err
					} else {
						return int64(newValue), nil
					}
				} else {
					return 0, err
				}
			}

			return value, nil
		} else {
			return 0, err
		}
	} else {
		return int64(newValue), nil
	}
}

// DecrFloat 浮点数自减，鉴于memcache不支持浮点数，这里通过整数自减来实现
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value float64 自减步长
// @return @1 float64 自减后的值
// @return @2 error 错误信息
func (c *Cache) DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	if newValue, err := c.DecrInt(ctx, key, int64(value)); err != nil {
		return 0, err
	} else {
		return float64(newValue), nil
	}
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
// @return @1 any 底层 Memcache 客户端
func (c *Cache) Client() any {
	if err := c.check(); err != nil {
		return nil
	}

	return c.opts.client
}

// check 检查缓存是否已关闭
func (c *Cache) check() error {
	if c.closed.Load() {
		return errors.ErrCacheClosed
	}

	return nil
}

// Close 关闭客户端
// @return @1 error 错误信息
func (c *Cache) Close() (err error) {
	if c.closed.Swap(true) {
		return errors.ErrCacheClosed
	}

	if c.builtin && c.opts.client != nil {
		return c.opts.client.Close()
	}

	return nil
}
