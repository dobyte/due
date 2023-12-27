package cache

import (
	"context"
	"time"
)

type Number interface {
	int | int8 | int16 | int32 | int64 | float32 | float64
}

type Cache interface {
	// Has 检测缓存是否存在
	Has(ctx context.Context, key string) (bool, error)
	// Get 获取缓存值
	Get(ctx context.Context, key string, def ...interface{}) Result
	// Set 设置缓存值
	Set(ctx context.Context, key string, value interface{}, expiration ...time.Duration) error
	// IncrInt 整数自增
	IncrInt(ctx context.Context, key string, value int64) (int64, error)
	// IncrFloat 浮点数自增
	IncrFloat(ctx context.Context, key string, value float64) (float64, error)
	// DecrInt 整数自减
	DecrInt(ctx context.Context, key string, value int64) (int64, error)
	// DecrFloat 浮点数自减
	DecrFloat(ctx context.Context, key string, value float64) (float64, error)
}
