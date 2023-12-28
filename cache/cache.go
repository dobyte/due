package cache

import (
	"context"
	"time"
)

type SetValueFunc func() (interface{}, time.Duration, error)

type Cache interface {
	// Has 检测缓存是否存在
	Has(ctx context.Context, key string) (bool, error)
	// Get 获取缓存值
	Get(ctx context.Context, key string, def ...interface{}) Result
	// Set 设置缓存值
	Set(ctx context.Context, key string, value interface{}, expiration ...time.Duration) error
	// GetSet 获取设置缓存值
	GetSet(ctx context.Context, key string, fn SetValueFunc) Result
	// Delete 删除缓存
	Delete(ctx context.Context, key string) (bool, error)
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
	Client() interface{}
}
