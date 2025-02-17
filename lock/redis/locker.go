package redis

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
func (l *Locker) Acquire(ctx context.Context, try ...bool) error {
	if err := l.maker.acquire(ctx, l.key, l.version, try...); err != nil {
		return err
	}

	l.timer = time.AfterFunc(l.maker.opts.expiration/2, l.renewal)

	return nil
}

// Release 释放锁
func (l *Locker) Release(ctx context.Context) error {
	var ok bool

	l.rw.RLock()
	if ok = l.timer != nil; ok {
		l.timer.Stop()
	}
	l.rw.RUnlock()

	if !ok {
		return nil
	}

	if err := l.maker.release(ctx, l.key, l.version); err != nil {
		return err
	}

	return nil
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
