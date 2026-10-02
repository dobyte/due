package cache

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

var globalCache Cache

// SetValueFunc is the callback used to generate a cache value. It returns the generated value and
// an error.
type SetValueFunc func() (any, error)

// Cache is the interface implemented by cache backends.
type Cache interface {
	// Has reports whether the entry identified by key exists.
	Has(ctx context.Context, key string) (bool, error)
	// Get returns the value of key. The optional def is returned when the entry does not exist.
	Get(ctx context.Context, key string, def ...any) Result
	// Set stores value under key. The optional expiration controls the entry's time to live: when
	// it is omitted a random duration within the configured expiration range is used, a value
	// greater than 0 sets an explicit expiration, -1 keeps the existing expiration and a value
	// less than -1 makes the entry never expire.
	Set(ctx context.Context, key string, value any, expiration ...time.Duration) error
	// GetSet returns the value of key, generating it with fn and storing it when the entry does not
	// exist.
	GetSet(ctx context.Context, key string, fn SetValueFunc) Result
	// Delete removes the given keys and reports how many keys were actually deleted.
	Delete(ctx context.Context, keys ...string) (int64, error)
	// IncrInt increments the integer stored at key by value and returns the resulting value.
	IncrInt(ctx context.Context, key string, value int64) (int64, error)
	// IncrFloat increments the float stored at key by value and returns the resulting value.
	IncrFloat(ctx context.Context, key string, value float64) (float64, error)
	// DecrInt decrements the integer stored at key by value and returns the resulting value.
	DecrInt(ctx context.Context, key string, value int64) (int64, error)
	// DecrFloat decrements the float stored at key by value and returns the resulting value.
	DecrFloat(ctx context.Context, key string, value float64) (float64, error)
	// AddPrefix returns key prefixed with the cache's key prefix.
	AddPrefix(key string) string
	// Client returns the underlying cache client.
	Client() any
	// Close closes the cache.
	Close() error
}

// SetCache sets the global cache instance. It closes the previously installed instance, if any,
// and ignores a nil cache.
func SetCache(cache Cache) {
	if cache == nil {
		log.Warn("cannot set a nil cache")
		return
	}

	if globalCache != nil {
		if err := globalCache.Close(); err != nil {
			log.Error("close cache failed: %v", err)
		}
	}

	globalCache = cache
}

// GetCache returns the global cache instance.
func GetCache() Cache {
	return globalCache
}

// Has reports whether the entry identified by key exists in the global cache.
func Has(ctx context.Context, key string) (bool, error) {
	if globalCache == nil {
		return false, errors.ErrMissingCacheInstance
	}

	return globalCache.Has(ctx, key)
}

// Get returns the value of key from the global cache. The optional def is returned when the entry
// does not exist.
func Get(ctx context.Context, key string, def ...any) Result {
	if globalCache == nil {
		return NewResult(nil, errors.ErrMissingCacheInstance)
	}

	return globalCache.Get(ctx, key, def...)
}

// Set stores value under key in the global cache. The optional expiration controls the entry's
// time to live: when it is omitted a random duration within the configured expiration range is
// used, a value greater than 0 sets an explicit expiration, -1 keeps the existing expiration and
// a value less than -1 makes the entry never expire.
func Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	if globalCache == nil {
		return errors.ErrMissingCacheInstance
	}

	return globalCache.Set(ctx, key, value, expiration...)
}

// GetSet returns the value of key from the global cache, generating it with fn and storing it when
// the entry does not exist.
func GetSet(ctx context.Context, key string, fn SetValueFunc) Result {
	if globalCache == nil {
		return NewResult(nil, errors.ErrMissingCacheInstance)
	}

	return globalCache.GetSet(ctx, key, fn)
}

// Delete removes the given keys from the global cache and reports how many keys were actually
// deleted.
func Delete(ctx context.Context, keys ...string) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.Delete(ctx, keys...)
}

// IncrInt increments the integer stored at key in the global cache by value and returns the
// resulting value.
func IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.IncrInt(ctx, key, value)
}

// IncrFloat increments the float stored at key in the global cache by value and returns the
// resulting value.
func IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.IncrFloat(ctx, key, value)
}

// DecrInt decrements the integer stored at key in the global cache by value and returns the
// resulting value.
func DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.DecrInt(ctx, key, value)
}

// DecrFloat decrements the float stored at key in the global cache by value and returns the
// resulting value.
func DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.DecrFloat(ctx, key, value)
}

// AddPrefix returns key prefixed with the global cache's key prefix.
func AddPrefix(key string) string {
	if globalCache == nil {
		return ""
	}

	return globalCache.AddPrefix(key)
}

// Client returns the underlying client of the global cache.
func Client() any {
	if globalCache == nil {
		return nil
	}

	return globalCache.Client()
}

// Close closes the global cache.
func Close() error {
	if globalCache == nil {
		return nil
	}

	return globalCache.Close()
}
