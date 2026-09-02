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

// Maker 锁构建器
// 基于 memcached 的 Add/Get/CompareAndSwap 原语制造分布式锁，
// 内部持有 memcached 客户端与全局锁配置，并为每把锁生成独立的版本标识(version)以校验锁所有权
type Maker struct {
	opts    *options
	builtin bool
}

// NewMaker 创建锁构建器
// 初始化锁配置项：未显式配置过期时间时采用默认值，并将小于1秒的过期时间收敛为1秒，
// 避免 memcached 将亚秒级过期时间截断为0(永不过期)；未提供外部客户端时自动创建内建客户端，
// 该内建客户端将随 Close 一并关闭
// @param opts ...Option 锁配置项
// @return @1 *Maker 锁构建器实例
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
// 依据锁名称拼接配置的前缀生成 memcached key，并为该锁生成唯一的版本标识；
// 每个 Locker 持有独立的版本标识，从而可在同一把锁上公平竞争
// @param name string 锁名称
// @return @1 lock.Locker 分布式锁
func (m *Maker) Make(name string) lock.Locker {
	l := &Locker{}
	l.maker = m
	l.version = xuuid.UUID()
	l.cancel.Store(context.CancelFunc(nil))

	if m.opts.prefix == "" {
		l.key = name
	} else {
		l.key = m.opts.prefix + ":" + name
	}

	return l
}

// Close 关闭构建器
// 仅关闭内建客户端；使用外部客户端时，其生命周期由外部调用方管理，此处不做处理
// @return @1 error 关闭失败时返回的错误
func (m *Maker) Close() error {
	if m.builtin {
		return m.opts.client.Close()
	}

	return nil
}

// 循环获取锁
// 以 Add 原语周期性地尝试写入锁，写入成功即代表获取成功；写入失败(ErrNotStored)说明锁已被他人持有，
// 按配置的间隔与最大重试次数循环重试，直至成功、重试次数耗尽或 ctx 被取消
// @param ctx context.Context 上下文，取消后立即终止获取
// @param key string memcached键
// @param version string 锁版本标识
// @return @1 error 获取成功返回nil；重试耗尽返回errors.ErrDeadlineExceeded；ctx被取消返回ctx.Err()
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
// 仅执行一次 Add 写入，不等待也不重试；可通过 expiration 指定固定过期时间，未指定时采用配置的默认过期时间
// @param key string memcached键
// @param version string 锁版本标识
// @param expiration ...time.Duration 可选的固定过期时间；为空或小于等于0时采用默认过期时间
// @return @1 error 获取成功返回nil；锁已被他人持有返回errors.ErrIllegalOperation
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
// 复用"读取-校验-CAS"流程，将过期时间改写为固定且足够古老的绝对时间戳(见releaseExpiration)，
// 使 memcached 立即判定该键已过期并删除，从而实现释放
// @param ctx context.Context 上下文
// @param key string memcached键
// @param version string 锁版本标识
// @return @1 error 释放成功返回nil；锁已丢失或所有权已变更返回errors.ErrIllegalOperation
func (m *Maker) release(ctx context.Context, key, version string) error {
	return m.swap(ctx, key, version, releaseExpiration)
}

// 执行续租锁操作
// 通过"读取-校验-CAS"将锁的过期时间刷新为配置的过期时长；首次操作失败时按指数退避
// (共3次，间隔100ms~1s)重试，期间一旦锁所有权丢失(返回errors.ErrIllegalOperation)则立即终止
// @param ctx context.Context 上下文
// @param key string memcached键
// @param version string 锁版本标识
// @return @1 error 续租成功返回nil；锁已丢失或所有权已变更返回errors.ErrIllegalOperation
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
// @param expiration time.Duration 锁过期时长
// @return @1 int32 memcached 过期秒数
func expirationSeconds(expiration time.Duration) int32 {
	if expiration <= 0 {
		return 0
	}

	return int32(max(int64(1), (expiration.Milliseconds()+999)/1000))
}

// 执行替换操作
// 操作流程为"读取-校验-CAS"，与同一把锁的续租/释放操作并发时可能产生 CAS 冲突，
// 冲突后重新读取再试；其余非预期结果统一映射为 ErrIllegalOperation，避免底层错误外泄
// @param key string memcached键
// @param version string 锁版本标识
// @param expiration int32 新的过期时间
// @return @1 error 替换成功返回nil；锁不存在或所有权已变更返回errors.ErrIllegalOperation
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
