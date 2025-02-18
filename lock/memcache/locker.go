package memcache

import (
	"context"
	"sync"
	"time"
)

type Locker struct {
	maker   *Maker
	key     string
	version string
	rw      sync.RWMutex
	timer   *time.Timer
}

// Acquire 获取锁
func (l *Locker) Acquire(ctx context.Context) error {
	if err := l.maker.acquire(ctx, l.key, l.version); err != nil {
		return err
	}

	l.timer = time.AfterFunc(l.maker.opts.expiration/2, l.renewal)

	return nil
}

// TryAcquire 尝试获取锁
func (l *Locker) TryAcquire(ctx context.Context, expiration ...time.Duration) error {
	return l.maker.tryAcquire(ctx, l.key, l.version, expiration...)
}

// Release 释放锁
func (l *Locker) Release(ctx context.Context) error {
	l.rw.RLock()
	if l.timer != nil {
		l.timer.Stop()
	}
	l.rw.RUnlock()

	return l.maker.release(ctx, l.key, l.version)
}

// 续租锁
func (l *Locker) renewal() {
	if err := l.maker.renewal(context.Background(), l.key, l.version); err != nil {
		return
	}

	l.rw.Lock()
	l.timer = time.AfterFunc(l.maker.opts.expiration/2, l.renewal)
	l.rw.Unlock()
}
