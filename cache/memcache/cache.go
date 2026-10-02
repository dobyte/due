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

// NewCache returns a new Memcache-backed Cache. The optional opts override the default
// configuration.
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

// Has reports whether the entry identified by key exists.
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

// Get returns the value of key. The optional def is returned when the entry does not exist.
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

// Set stores value under key. The optional expiration sets the entry's time to live; when it is
// omitted a random duration within the configured expiration range is used.
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

// GetSet returns the value of key, generating it with fn and storing it when the entry does not
// exist.
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

// Delete removes the given keys and reports how many keys were actually deleted.
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

// IncrInt increments the integer stored at key by value and returns the resulting value.
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

// IncrFloat increments the float stored at key by value and returns the resulting value. Since
// Memcache does not support floats, the increment is implemented through integer increments.
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

// DecrInt decrements the integer stored at key by value and returns the resulting value.
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

// DecrFloat decrements the float stored at key by value and returns the resulting value. Since
// Memcache does not support floats, the decrement is implemented through integer decrements.
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

// AddPrefix returns key prefixed with the cache's key prefix.
func (c *Cache) AddPrefix(key string) string {
	if c.opts.prefix == "" {
		return key
	} else {
		return c.opts.prefix + ":" + key
	}
}

// Client returns the underlying Memcache client.
func (c *Cache) Client() any {
	if err := c.check(); err != nil {
		return nil
	}

	return c.opts.client
}

// check reports an error when the cache has been closed.
func (c *Cache) check() error {
	if c.closed.Load() {
		return errors.ErrCacheClosed
	}

	return nil
}

// Close closes the cache. It reports [errors.ErrCacheClosed] when the cache is already closed. The
// built-in client is closed only when it was created by [NewCache].
func (c *Cache) Close() (err error) {
	if c.closed.Swap(true) {
		return errors.ErrCacheClosed
	}

	if c.builtin && c.opts.client != nil {
		return c.opts.client.Close()
	}

	return nil
}
