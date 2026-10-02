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

// NewCache returns a new Redis-backed Cache. The optional opts override the default configuration.
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

// Has reports whether the entry identified by key exists.
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

// Get returns the value of key. The optional def is returned when the entry does not exist.
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

// Set stores value under key. The optional expiration controls the entry's time to live: when it
// is omitted a random duration within the configured expiration range is used, a value greater
// than 0 sets an explicit expiration, -1 keeps the existing expiration and a value less than -1
// makes the entry never expire.
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

// GetSet returns the value of key, generating it with fn and storing it when the entry does not
// exist.
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

// Delete removes the given keys and reports how many keys were actually deleted.
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

// IncrInt increments the integer stored at key by value and returns the resulting value.
func (c *Cache) IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.IncrBy(ctx, c.AddPrefix(key), value).Result()
}

// IncrFloat increments the float stored at key by value and returns the resulting value.
func (c *Cache) IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.IncrByFloat(ctx, c.AddPrefix(key), value).Result()
}

// DecrInt decrements the integer stored at key by value and returns the resulting value.
func (c *Cache) DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.DecrBy(ctx, c.AddPrefix(key), value).Result()
}

// DecrFloat decrements the float stored at key by value and returns the resulting value.
func (c *Cache) DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}

	return c.opts.client.IncrByFloat(ctx, c.AddPrefix(key), -value).Result()
}

// AddPrefix returns key prefixed with the cache's key prefix.
func (c *Cache) AddPrefix(key string) string {
	if c.opts.prefix == "" {
		return key
	} else {
		return c.opts.prefix + ":" + key
	}
}

// Client returns the underlying Redis client.
func (c *Cache) Client() any {
	if err := c.check(); err != nil {
		return nil
	}

	return c.opts.client
}

// check reports a construction error or an error when the cache has been closed.
func (c *Cache) check() error {
	if c.err != nil {
		return c.err
	}

	if c.closed.Load() {
		return errors.ErrCacheClosed
	}

	return nil
}

// Close closes the cache. It reports [errors.ErrCacheClosed] when the cache is already closed. The
// built-in client is closed only when it was created by [NewCache].
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
