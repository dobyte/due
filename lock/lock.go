package lock

import "context"

var globalMaker Maker

type Maker interface {
	// Make 制造一个Locker
	Make(name string) Locker
}

type Locker interface {
	// Acquire 获取锁
	Acquire(ctx context.Context) error
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
