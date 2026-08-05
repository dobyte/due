package redis

import (
	"context"
	"time"
)

type Locker struct {
	maker   *Maker
	key     string
	version string
	cancel  context.CancelFunc
}

// Acquire 获取锁
func (l *Locker) Acquire(ctx context.Context) error {
	if err := l.maker.acquire(ctx, l.key, l.version); err != nil {
		return err
	}

	l.renewal()

	return nil
}

// TryAcquire 尝试获取锁
// expiration 用于设置固定过期时间
func (l *Locker) TryAcquire(ctx context.Context, expiration ...time.Duration) error {
	if err := l.maker.tryAcquire(ctx, l.key, l.version, expiration...); err != nil {
		return err
	}

	if len(expiration) == 0 || expiration[0] <= 0 {
		l.renewal()
	}

	return nil
}

// Release 释放锁
func (l *Locker) Release(ctx context.Context) error {
	if l.cancel != nil {
		l.cancel()
	}

	return l.maker.release(ctx, l.key, l.version)
}

// 续租锁
func (l *Locker) renewal() {
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel

	go func() {
		ticker := time.NewTicker(l.maker.opts.expiration / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := l.maker.renewal(ctx, l.key, l.version); err != nil {
					return
				}
			}
		}
	}()
}
