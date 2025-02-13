package redis

import "context"

type Locker struct {
	maker   *Maker
	name    string
	version string
}

// Acquire 获取锁
func (l *Locker) Acquire(ctx context.Context) error {

}

// Release 释放锁
func (l *Locker) Release(ctx context.Context) error {
	return l.maker.release(ctx, l.name, l.version)
}
