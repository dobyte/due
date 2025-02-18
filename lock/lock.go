package lock

import (
	"context"
	"time"
)

var globalMaker Maker

type Maker interface {
	// Make 制造一个Locker
	Make(name string) Locker
	// Close 关闭构建器
	Close() error
}

type Option struct {
	Once       bool          // 是否仅获取一次；默认阻塞地获取，直到获取成功
	Expiration time.Duration //
}

type Locker interface {
	// Acquire 获取锁
	Acquire(ctx context.Context) error
	// TryAcquire 尝试获取锁
	TryAcquire(ctx context.Context, expiration ...time.Duration) error
	// Release 释放锁
	Release(ctx context.Context) error
}

// SetMaker 设置Locker制造商
func SetMaker(maker Maker) {
	globalMaker = maker
}

// GetMaker 获取Locker制造商
func GetMaker() Maker {
	return globalMaker
}

// Make 制造一个Locker
func Make(name string) Locker {
	if globalMaker != nil {
		return globalMaker.Make(name)
	} else {
		return nil
	}
}

// Close 关闭构建器
func Close() error {
	if globalMaker != nil {
		return globalMaker.Close()
	} else {
		return nil
	}
}
