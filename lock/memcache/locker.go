package memcache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// renewalRetryInterval 续租遇瞬时故障后的补偿重试间隔
// 故障期间按此短间隔持续重试(而非等待完整续租周期)，确保服务在锁过期前恢复时能尽快续租成功；
// 同时作为过期时间过短、退避预算不足时的忙等保护间隔
const renewalRetryInterval = 100 * time.Millisecond

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
// @return @1 error 释放成功返回nil；锁已丢失或所有权已变更返回errors.ErrIllegalOperation；持续CAS冲突(瞬时竞争)时返回驱动错误
func (l *Locker) Release(ctx context.Context) error {
	// 原子换出续租取消函数并停止续租；
	// 无续租(如 TryAcquire 指定了固定过期时间)时换出的为类型化 nil
	if prev := l.cancel.Swap(context.CancelFunc(nil)).(context.CancelFunc); prev != nil {
		prev()
	}

	return l.maker.release(ctx, l.key, l.version)
}

// 续租锁
// 启动后台协程周期性续租：正常情况下每隔锁过期时间的一半续租一次；
// 续租遇到瞬时故障时，一次续租内部先做指数退避重试(见maker.renewal)，退避耗尽后缩短为
// 短间隔(renewalRetryInterval)再次进入续租，从而在故障恢复后尽快续租成功，避免干等固定周期浪费恢复窗口；
// 锁丢失(返回errors.ErrIllegalOperation)或续租被取消(Release/Close)时协程退出。
// 每次调用会先取消上一次续租的取消函数，确保同一 Locker 同时至多存在一个续租协程。
// 协程经由 xcall.Go 启动，执行过程中的 panic 会被自动捕获，避免拖垮进程
func (l *Locker) renewal() {
	// 续租 ctx 派生自 Maker 的上下文，Maker.Close 时会一并取消所有续租协程
	ctx, cancel := context.WithCancel(l.maker.ctx)

	if prev := l.cancel.Swap(cancel).(context.CancelFunc); prev != nil {
		prev()
	}

	xcall.Go(func() {
		var (
			interval = l.maker.opts.expiration / 2 // 正常续租周期
			delay    = interval                    // 下一次续租的等待时间
			warned   bool
		)

		for {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			switch err := l.maker.renewal(ctx, l.key, l.version); {
			case err == nil: // 续租成功，恢复正常续租周期
				warned = false
				delay = interval
			case ctx.Err() != nil: // Release/Close 已取消续租(正常退出路径)，无需告警
				return
			case errors.Is(err, errors.ErrIllegalOperation): // 锁已过期或所有权已变更，继续续租已无意义
				log.Warnf("renew lock failed, the lock has been lost: %v", err)
				return
			default: // 瞬时故障：缩短等待间隔持续补偿重试，在锁过期前尽快续租成功；仅告警一次，续租成功后恢复正常周期
				if !warned {
					log.Warnf("renew lock failed, will retry in a short interval: %v", err)
					warned = true
				}

				delay = renewalRetryInterval
			}
		}
	})
}
