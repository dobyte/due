package redis

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Locker struct {
	maker   *Maker
	key     string
	version string
	rw      sync.RWMutex
	timer   atomic.Value
}

// Acquire 获取锁
func (l *Locker) Acquire(ctx context.Context) error {
	if err := l.maker.acquire(ctx, l.key, l.version); err != nil {
		return err
	}

	l.timer.Store(time.AfterFunc(l.maker.opts.expiration/2, l.renewal))

	return nil
}

// TryAcquire 尝试获取锁
func (l *Locker) TryAcquire(ctx context.Context, expiration ...time.Duration) error {
	if err := l.maker.tryAcquire(ctx, l.key, l.version, expiration...); err != nil {
		return err
	}

	if len(expiration) == 0 {
		l.timer.Store(time.AfterFunc(l.maker.opts.expiration/2, l.renewal))
	}

	return nil
}

// Release 释放锁
func (l *Locker) Release(ctx context.Context) error {
	timer := l.timer.Swap((*time.Timer)(nil))

	if t, ok := timer.(*time.Timer); ok && t != nil {
		t.Stop()
	}

	return l.maker.release(ctx, l.key, l.version)
}

// 续租锁
func (l *Locker) renewal() {
	if err := l.maker.renewal(context.Background(), l.key, l.version); err != nil {
		return
	}

	l.timer.Store(time.AfterFunc(l.maker.opts.expiration/2, l.renewal))
}
