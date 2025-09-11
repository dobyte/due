package cache

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

var globalCache Cache

type SetValueFunc func() (any, error)

type Cache interface {
	// Has 检测缓存是否存在
	Has(ctx context.Context, key string) (bool, error)
	// Get 获取缓存值
	Get(ctx context.Context, key string, def ...any) Result
	// Set 设置缓存值
	Set(ctx context.Context, key string, value any, expiration ...time.Duration) error
	// GetSet 获取设置缓存值
	GetSet(ctx context.Context, key string, fn SetValueFunc) Result
	// Delete 删除缓存
	Delete(ctx context.Context, keys ...string) (int64, error)
	// IncrInt 整数自增
	IncrInt(ctx context.Context, key string, value int64) (int64, error)
	// IncrFloat 浮点数自增
	IncrFloat(ctx context.Context, key string, value float64) (float64, error)
	// DecrInt 整数自减
	DecrInt(ctx context.Context, key string, value int64) (int64, error)
	// DecrFloat 浮点数自减
	DecrFloat(ctx context.Context, key string, value float64) (float64, error)
	// AddPrefix 添加Key前缀
	AddPrefix(key string) string
	// Client 获取客户端
	Client() any
	// Close 关闭缓存
	Close() error
	// Ping 检查缓存连接
	Ping(ctx context.Context) error
}

// SetCache 设置缓存
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

// GetCache 获取缓存
func GetCache() Cache {
	return globalCache
}

// Has 检测缓存是否存在
func Has(ctx context.Context, key string) (bool, error) {
	if globalCache == nil {
		return false, errors.ErrMissingCacheInstance
	}

	return globalCache.Has(ctx, key)
}

// Get 获取缓存值
func Get(ctx context.Context, key string, def ...any) Result {
	if globalCache == nil {
		return NewResult(nil, errors.ErrMissingCacheInstance)
	}

	return globalCache.Get(ctx, key, def...)
}

// Set 设置缓存值
func Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	if globalCache == nil {
		return errors.ErrMissingCacheInstance
	}

	return globalCache.Set(ctx, key, value, expiration...)
}

// GetSet 获取设置缓存值
func GetSet(ctx context.Context, key string, fn SetValueFunc) Result {
	if globalCache == nil {
		return NewResult(nil, errors.ErrMissingCacheInstance)
	}

	return globalCache.GetSet(ctx, key, fn)
}

// Delete 删除缓存
func Delete(ctx context.Context, keys ...string) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.Delete(ctx, keys...)
}

// IncrInt 整数自增
func IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.IncrInt(ctx, key, value)
}

// IncrFloat 浮点数自增
func IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.IncrFloat(ctx, key, value)
}

// DecrInt 整数自减
func DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.DecrInt(ctx, key, value)
}

// DecrFloat 浮点数自减
func DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.DecrFloat(ctx, key, value)
}

// AddPrefix 添加Key前缀
func AddPrefix(key string) string {
	if globalCache == nil {
		return ""
	}

	return globalCache.AddPrefix(key)
}

// Client 获取客户端
func Client() any {
	if globalCache == nil {
		return nil
	}

	return globalCache.Client()
}

// Close 关闭缓存
func Close() error {
	if globalCache == nil {
		return nil
	}

	return globalCache.Close()
}

// Ping 检查缓存连接
func Ping(ctx context.Context) error {
	return globalCache.Ping(ctx)
}
