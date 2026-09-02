package memcache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

type Locker struct {
	maker   *Maker
	key     string
	version string
	cancel  atomic.Value // 保存续租的取消函数 context.CancelFunc，未续租时其值为类型化 nil
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
	// 原子换出续租取消函数并停止续租；
	// 无续租(如 TryAcquire 指定了固定过期时间)时换出的为类型化 nil
	if prev := l.cancel.Swap(context.CancelFunc(nil)).(context.CancelFunc); prev != nil {
		prev()
	}

	return l.maker.release(ctx, l.key, l.version)
}

// 续租锁
func (l *Locker) renewal() {
	ctx, cancel := context.WithCancel(context.Background())

	// 原子换出上一次续租的取消函数并取消之，防止同一 Locker 被重复获取时产生多个续租协程；
	// 取消函数幂等，被换出后旧的续租协程会在下一个周期观测到 ctx 取消而退出
	if prev := l.cancel.Swap(cancel).(context.CancelFunc); prev != nil {
		prev()
	}

	go func() {
		ticker := time.NewTicker(l.maker.opts.expiration / 2)
		defer ticker.Stop()

		var warned bool

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := l.maker.renewal(ctx, l.key, l.version); err != nil {
					// Release 已取消续租(正常释放路径)，无需告警
					if ctx.Err() != nil {
						return
					}

					if errors.Is(err, errors.ErrIllegalOperation) {
						// 锁已过期或所有权已变更，继续续租已无意义
						log.Warnf("renew lock failed, the lock has been lost: %v", err)
						return
					}

					// 瞬时故障：仅告警一次，随后等待下一个周期继续尝试续租，
					// 以便服务恢复后(锁尚未过期时)能够继续持有锁
					if !warned {
						log.Warnf("renew lock failed, will retry at the next interval: %v", err)
						warned = true
					}

					continue
				}

				warned = false
			}
		}
	}()
}
