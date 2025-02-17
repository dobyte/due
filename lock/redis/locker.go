package redis

import (
	"context"
	"sync"
	"time"
)

type Locker struct {
	maker   *Maker
	name    string
	version string
	rw      sync.RWMutex
	timer   *time.Timer
}

// Acquire 获取锁
func (l *Locker) Acquire(ctx context.Context) error {
	if err := l.maker.acquire(ctx, l.name, l.version); err != nil {
		return err
	}

	l.timer = time.AfterFunc(l.maker.opts.expiration/2, l.renewal)

	return nil
}

// Release 释放锁
func (l *Locker) Release(ctx context.Context) error {
	if err := l.maker.release(ctx, l.name, l.version); err != nil {
		return err
	}

	l.rw.RLock()
	l.timer.Stop()
	l.rw.RUnlock()

	return nil
}

// 续租锁
func (l *Locker) renewal() {
	if err := l.maker.renewal(context.Background(), l.name, l.version); err != nil {
		return
	}

	l.rw.Lock()
	l.timer = time.AfterFunc(l.maker.opts.expiration/2, l.renewal)
	l.rw.Unlock()
}
