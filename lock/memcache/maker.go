package memcache

import (
	"context"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
)

const (
	// 释放锁时写入的过期时间戳
	// memcached 将超过 30 天的过期时间解析为绝对时间戳(Unix秒)，
	// 且当该时间戳早于服务器启动时间时会钳制为"立即过期"，据此实现释放锁的效果。
	// 这里使用固定且足够古老的绝对时间戳(2001年)，确保任意服务器的启动时间都晚于该值；
	// 不可使用相对当前时间的偏移量(如 now-1年)，否则服务器运行时长一旦超过该偏移，
	// 时间戳会被当作未来的绝对时间，导致锁无法释放
	releaseExpiration = int32(1000000000)

	// CAS 冲突的最大重试次数
	// 锁的续租与释放共用"读取-校验-CAS"流程，同一把锁的并发操作(如释放与在途续租)
	// 可能产生 CAS 冲突，冲突后重新读取并重试即可解决
	maxSwapRetries = 5
)

type Maker struct {
	opts    *options
	builtin bool
}

func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.expiration <= 0 {
		o.expiration = xconv.Duration(defaultExpiration)
	}

	// memcached 的过期时间精度为 1 秒，0 表示永不过期；
	// 将过期时间下限收敛为 1s，避免亚秒级配置被截断为 0 导致锁永不过期
	if o.expiration < time.Second {
		o.expiration = time.Second
	}

	m := &Maker{}
	m.opts = o

	if o.client == nil {
		o.client = memcache.New(o.addrs...)
		m.builtin = true
	}

	return m
}

// Make 制造一个Locker
func (m *Maker) Make(name string) lock.Locker {
	l := &Locker{}
	l.maker = m
	l.version = xuuid.UUID()

	// 预置类型化的空取消函数，完成 cancel 原子值的初始化。
	// atomic.Value 未初始化时首次 Swap 会返回无类型 nil，其后的类型断言将 panic，
	// 因此所有 Locker 都必须先经过此处初始化
	l.cancel.Store(context.CancelFunc(nil))

	if m.opts.prefix == "" {
		l.key = name
	} else {
		l.key = m.opts.prefix + ":" + name
	}

	return l
}

// Close 关闭构建器
func (m *Maker) Close() error {
	if m.builtin {
		return m.opts.client.Close()
	}

	return nil
}

// 执行获取锁操作
func (m *Maker) acquire(ctx context.Context, key, version string) error {
	var (
		err     error
		retries int
		item    = &memcache.Item{
			Key:        key,
			Value:      xconv.Bytes(version),
			Expiration: expirationSeconds(m.opts.expiration),
		}
	)

	for {
		if err = m.opts.client.Add(item); err == nil {
			return nil
		}

		if !errors.Is(err, memcache.ErrNotStored) {
			return err
		}

		if m.opts.acquireMaxRetries > 0 {
			if retries >= m.opts.acquireMaxRetries {
				return errors.ErrDeadlineExceeded
			}

			retries++
		}

		ticker := time.NewTimer(m.opts.acquireInterval)
		select {
		case <-ctx.Done():
			ticker.Stop()
			return ctx.Err()
		case <-ticker.C:
			ticker.Stop()
		}
	}
}

// 尝试获取锁
func (m *Maker) tryAcquire(_ context.Context, key, version string, expiration ...time.Duration) error {
	item := &memcache.Item{Key: key, Value: xconv.Bytes(version)}

	if len(expiration) > 0 && expiration[0] > 0 {
		item.Expiration = expirationSeconds(expiration[0])
	} else {
		item.Expiration = expirationSeconds(m.opts.expiration)
	}

	if err := m.opts.client.Add(item); err != nil {
		if errors.Is(err, memcache.ErrNotStored) {
			return errors.ErrIllegalOperation
		}

		return err
	}

	return nil
}

// 执行释放锁操作
func (m *Maker) release(ctx context.Context, key, version string) error {
	return m.swap(ctx, key, version, releaseExpiration)
}

// 执行续租锁操作
func (m *Maker) renewal(ctx context.Context, key, version string) error {
	expiration := expirationSeconds(m.opts.expiration)

	if err := m.swap(ctx, key, version, expiration); err == nil {
		return nil
	} else if errors.Is(err, errors.ErrIllegalOperation) {
		return err
	}

	return xcall.Backoff(ctx, func(ctx context.Context, attempt int) (bool, error) {
		err := m.swap(ctx, key, version, expiration)
		if err != nil && !errors.Is(err, errors.ErrIllegalOperation) {
			return true, err
		}

		return false, err
	}, 3, 100*time.Millisecond, time.Second)
}

// 将过期时间转换为 memcached 的过期秒数
// memcached 的过期时间精度为 1 秒，且 0 表示永不过期；
// 为避免亚秒级时长被截断为 0(永不过期)，统一向上取整，并保证最小值为 1 秒
func expirationSeconds(expiration time.Duration) int32 {
	if expiration <= 0 {
		return 0
	}

	return int32(max(int64(1), (expiration.Milliseconds()+999)/1000))
}

// 执行替换操作
// 操作流程为"读取-校验-CAS"，与同一把锁的续租/释放操作并发时可能产生 CAS 冲突，
// 冲突后重新读取再试；其余非预期结果统一映射为 ErrIllegalOperation，避免底层错误外泄
func (m *Maker) swap(_ context.Context, key, version string, expiration int32) error {
	for range maxSwapRetries {
		item, err := m.opts.client.Get(key)
		if err != nil {
			if errors.Is(err, memcache.ErrCacheMiss) {
				// 锁不存在，说明锁已过期或已被释放
				return errors.ErrIllegalOperation
			}

			return err
		}

		// 锁已被其他持有者(不同version)获取
		if xconv.String(item.Value) != version {
			return errors.ErrIllegalOperation
		}

		item.Expiration = expiration

		if err = m.opts.client.CompareAndSwap(item); err == nil {
			return nil
		}

		switch {
		case errors.Is(err, memcache.ErrCASConflict):
			// 与同锁的续租/释放操作竞争，重新读取后再试
		case errors.Is(err, memcache.ErrNotStored), errors.Is(err, memcache.ErrCacheMiss):
			// 锁在读取与交换之间已被删除或过期，所有权已丧失
			return errors.ErrIllegalOperation
		default:
			return err
		}
	}

	return errors.ErrIllegalOperation
}
