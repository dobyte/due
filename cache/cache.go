package cache

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

var globalCache Cache

// SetValueFunc 设置缓存值的回调函数类型
// @return @1 any 生成的缓存值
// @return @2 error 错误信息
type SetValueFunc func() (any, error)

type Cache interface {
	// Has 检测缓存是否存在
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @return @1 bool 缓存是否存在
	// @return @2 error 错误信息
	Has(ctx context.Context, key string) (bool, error)
	// Get 获取缓存值
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @param def ...any 可选默认值，当缓存不存在时返回该默认值
	// @return @1 Result 缓存结果
	Get(ctx context.Context, key string, def ...any) Result
	// Set 设置缓存值
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @param value any 缓存值
	// @param expiration ...time.Duration 过期时间，省略表示使用过期时间范围随机，>0表示使用具体的过期时间，=-1表示保持原有过期时间，<-1表示永不过期
	// @return @1 error 错误信息
	Set(ctx context.Context, key string, value any, expiration ...time.Duration) error
	// GetSet 获取缓存值，若不存在则通过 fn 生成后写入并返回
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @param fn SetValueFunc 缓存未命中时执行的回调，用于生成缓存值
	// @return @1 Result 缓存结果
	GetSet(ctx context.Context, key string, fn SetValueFunc) Result
	// Delete 删除缓存
	// @param ctx context.Context 上下文
	// @param keys ...string 缓存键，可传入多个
	// @return @1 int64 实际删除的 key 数量
	// @return @2 error 错误信息
	Delete(ctx context.Context, keys ...string) (int64, error)
	// IncrInt 整数自增
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @param value int64 自增步长
	// @return @1 int64 自增后的值
	// @return @2 error 错误信息
	IncrInt(ctx context.Context, key string, value int64) (int64, error)
	// IncrFloat 浮点数自增
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @param value float64 自增步长
	// @return @1 float64 自增后的值
	// @return @2 error 错误信息
	IncrFloat(ctx context.Context, key string, value float64) (float64, error)
	// DecrInt 整数自减
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @param value int64 自减步长
	// @return @1 int64 自减后的值
	// @return @2 error 错误信息
	DecrInt(ctx context.Context, key string, value int64) (int64, error)
	// DecrFloat 浮点数自减
	// @param ctx context.Context 上下文
	// @param key string 缓存键
	// @param value float64 自减步长
	// @return @1 float64 自减后的值
	// @return @2 error 错误信息
	DecrFloat(ctx context.Context, key string, value float64) (float64, error)
	// AddPrefix 添加Key前缀
	// @param key string 缓存键
	// @return @1 string 添加前缀后的完整键名
	AddPrefix(key string) string
	// Client 获取客户端
	// @return @1 any 底层缓存客户端
	Client() any
	// Close 关闭缓存
	// @return @1 error 错误信息
	Close() error
}

// SetCache 设置全局缓存实例
// @param cache Cache 缓存实例
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

// GetCache 获取全局缓存实例
// @return @1 Cache 缓存实例
func GetCache() Cache {
	return globalCache
}

// Has 检测缓存是否存在
// @param ctx context.Context 上下文
// @param key string 缓存键
// @return @1 bool 缓存是否存在
// @return @2 error 错误信息
func Has(ctx context.Context, key string) (bool, error) {
	if globalCache == nil {
		return false, errors.ErrMissingCacheInstance
	}

	return globalCache.Has(ctx, key)
}

// Get 获取缓存值
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param def ...any 可选默认值，当缓存不存在时返回该默认值
// @return @1 Result 缓存结果
func Get(ctx context.Context, key string, def ...any) Result {
	if globalCache == nil {
		return NewResult(nil, errors.ErrMissingCacheInstance)
	}

	return globalCache.Get(ctx, key, def...)
}

// Set 设置缓存值
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value any 缓存值
// @param expiration ...time.Duration 过期时间，省略表示使用过期时间范围随机，>0表示使用具体的过期时间，=-1表示保持原有过期时间，<-1表示永不过期
// @return @1 error 错误信息
func Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	if globalCache == nil {
		return errors.ErrMissingCacheInstance
	}

	return globalCache.Set(ctx, key, value, expiration...)
}

// GetSet 获取缓存值，若不存在则通过 fn 生成后写入并返回
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param fn SetValueFunc 缓存未命中时执行的回调，用于生成缓存值
// @return @1 Result 缓存结果
func GetSet(ctx context.Context, key string, fn SetValueFunc) Result {
	if globalCache == nil {
		return NewResult(nil, errors.ErrMissingCacheInstance)
	}

	return globalCache.GetSet(ctx, key, fn)
}

// Delete 删除缓存
// @param ctx context.Context 上下文
// @param keys ...string 缓存键，可传入多个
// @return @1 int64 实际删除的 key 数量
// @return @2 error 错误信息
func Delete(ctx context.Context, keys ...string) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.Delete(ctx, keys...)
}

// IncrInt 整数自增
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value int64 自增步长
// @return @1 int64 自增后的值
// @return @2 error 错误信息
func IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.IncrInt(ctx, key, value)
}

// IncrFloat 浮点数自增
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value float64 自增步长
// @return @1 float64 自增后的值
// @return @2 error 错误信息
func IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.IncrFloat(ctx, key, value)
}

// DecrInt 整数自减
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value int64 自减步长
// @return @1 int64 自减后的值
// @return @2 error 错误信息
func DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.DecrInt(ctx, key, value)
}

// DecrFloat 浮点数自减
// @param ctx context.Context 上下文
// @param key string 缓存键
// @param value float64 自减步长
// @return @1 float64 自减后的值
// @return @2 error 错误信息
func DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	if globalCache == nil {
		return 0, errors.ErrMissingCacheInstance
	}

	return globalCache.DecrFloat(ctx, key, value)
}

// AddPrefix 添加Key前缀
// @param key string 缓存键
// @return @1 string 添加前缀后的完整键名
func AddPrefix(key string) string {
	if globalCache == nil {
		return ""
	}

	return globalCache.AddPrefix(key)
}

// Client 获取客户端
// @return @1 any 底层缓存客户端
func Client() any {
	if globalCache == nil {
		return nil
	}

	return globalCache.Client()
}

// Close 关闭缓存
// @return @1 error 错误信息
func Close() error {
	if globalCache == nil {
		return nil
	}

	return globalCache.Close()
}
