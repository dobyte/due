package memcache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

// Locker 分布式锁
// 基于 memcached 实现的可续租分布式锁：每把锁对应一个 memcached key，由独立的版本标识(version)界定所有权，
// 持有期间锁的过期时间按"读取-校验-CAS"循环刷新，过期或丢失所有权后自动失效
type Locker struct {
	maker   *Maker
	key     string
	version string
	cancel  atomic.Value // 保存续租的取消函数 context.CancelFunc，未续租时其值为类型化 nil
}

// Acquire 获取锁
// 阻塞式获取锁，获取成功后自动开启后台续租以持续持有，直到主动释放或锁丢失
// @param ctx context.Context 上下文，取消后中断获取
// @return @1 error 获取成功返回nil；重试耗尽返回errors.ErrDeadlineExceeded；ctx被取消返回ctx.Err()
func (l *Locker) Acquire(ctx context.Context) error {
	if err := l.maker.acquire(ctx, l.key, l.version); err != nil {
		return err
	}

	l.renewal()

	return nil
}

// TryAcquire 尝试获取锁
// 仅尝试一次，获取失败立即返回、不阻塞等待；获取成功时，若指定了大于0的 expiration 则锁按该时长
// 一次性过期、不再续租，否则采用默认过期时间并自动开启后台续租
// @param ctx context.Context 上下文
// @param expiration ...time.Duration 可选的固定过期时间；为空或小于等于0时不限定期限并开启续租
// @return @1 error 获取成功返回nil；锁已被他人持有返回errors.ErrIllegalOperation
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
// 先停止后台续租，再按版本标识执行释放；仅锁的所有者(版本标识匹配)才能释放成功
// @param ctx context.Context 上下文
// @return @1 error 释放成功返回nil；锁已丢失或所有权已变更返回errors.ErrIllegalOperation
func (l *Locker) Release(ctx context.Context) error {
	// 原子换出续租取消函数并停止续租；
	// 无续租(如 TryAcquire 指定了固定过期时间)时换出的为类型化 nil
	if prev := l.cancel.Swap(context.CancelFunc(nil)).(context.CancelFunc); prev != nil {
		prev()
	}

	return l.maker.release(ctx, l.key, l.version)
}

// 续租锁
// 启动后台协程，每隔锁过期时间的一半执行一次续租，以持续持有锁，直到被释放或所有权丢失；
// 每次调用会先取消上一次续租的取消函数，确保同一 Locker 同时至多存在一个续租协程
func (l *Locker) renewal() {
	ctx, cancel := context.WithCancel(context.Background())

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
