package redis

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/redis/go-redis/v9"
)

// Maker 锁构建器
// 基于 redis 的 SET NX 原语与释放/续租 Lua 脚本制造分布式锁，
// 内部持有 redis 客户端与全局锁配置，并为每把锁生成独立的版本标识(version)以校验锁所有权
type Maker struct {
	err           error           // 初始化错误(如 TLS 配置失败)，非nil时所有操作直接返回该错误
	opts          *options        // 锁配置项
	builtin       bool            // 是否内建客户端，内建客户端将随 Close 一并关闭
	ctx           context.Context // 构建器生命周期上下文，Close 时取消，用于停止所有续租协程
	cancel        context.CancelFunc
	releaseScript *redis.Script // 释放锁的 Lua 脚本
	renewalScript *redis.Script // 续租锁的 Lua 脚本
}

// NewMaker 创建锁构建器
// 初始化锁配置项：未显式配置过期时间时采用默认值，并将不足1毫秒的过期时间收敛为1毫秒，
// 避免亚毫秒级配置被截断为0(Set 时不设过期、PEXPIRE 0 时直接删除锁)；
// 循环获取锁的间隔时间小于等于0时收敛为默认值，避免重试退化为无退避忙等循环；
// 未提供外部客户端时自动创建内建客户端，该内建客户端将随 Close 一并关闭
// @param opts ...Option 锁配置项
// @return @1 *Maker 锁构建器实例；初始化失败(如 TLS 配置错误)时，实例的获取/释放等操作将返回该错误
func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.expiration <= 0 {
		o.expiration = xconv.Duration(defaultExpiration)
	}

	// redis 的过期时间精度为 1 毫秒，0 表示不设置过期时间(而 PEXPIRE 0 会直接删除 key)；
	// 将过期时间下限收敛为 1 毫秒，避免亚毫秒级配置被截断为 0 导致锁永不过期或立即失效
	if o.expiration < time.Millisecond {
		o.expiration = time.Millisecond
	}

	// 获取锁的间隔时间必须大于0，0或负值会使重试定时器立即触发，退化为无退避忙等循环
	if o.acquireInterval <= 0 {
		o.acquireInterval = xconv.Duration(defaultAcquireInterval)
	}

	m := &Maker{}
	m.opts = o
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.releaseScript = redis.NewScript(releaseScript)
	m.renewalScript = redis.NewScript(renewalScript)

	if m.opts.client == nil {
		options := &redis.UniversalOptions{
			Addrs:      m.opts.addrs,
			DB:         m.opts.db,
			Username:   m.opts.username,
			Password:   m.opts.password,
			MaxRetries: m.opts.maxRetries,
		}

		if m.opts.certFile != "" && m.opts.keyFile != "" && m.opts.caFile != "" {
			if options.TLSConfig, m.err = tls.MakeRedisTLSConfig(m.opts.certFile, m.opts.keyFile, m.opts.caFile); m.err != nil {
				return m
			}
		}

		m.opts.client, m.builtin = redis.NewUniversalClient(options), true
	}

	return m
}

// Make 制造一个Locker
// 依据锁名称拼接配置的前缀生成 redis key，并为该锁生成唯一的版本标识；
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
// 先取消构建器生命周期上下文以停止所有后台续租协程，再关闭内建客户端；
// 使用外部客户端时，其生命周期由外部调用方管理，此处不做处理
// @return @1 error 关闭失败时返回的错误
func (m *Maker) Close() error {
	// 停止所有后台续租协程；CancelFunc 幂等，可重复调用
	m.cancel()

	if m.err != nil {
		return m.err
	}

	if m.builtin {
		return m.opts.client.Close()
	}

	return nil
}

// 循环获取锁
// 以 SET NX 原语周期性地尝试写入锁，写入成功即代表获取成功；写入失败(返回redis.Nil)说明锁已被他人持有，
// 按配置的间隔与最大重试次数循环重试，直至成功、重试次数耗尽或 ctx 被取消
// @param ctx context.Context 上下文，取消后立即终止获取
// @param key string redis键
// @param version string 锁版本标识
// @return @1 error 获取成功返回nil；重试耗尽返回errors.ErrDeadlineExceeded；ctx被取消返回ctx.Err()；构建器已关闭返回errors.ErrIllegalOperation
func (m *Maker) acquire(ctx context.Context, key, version string) error {
	if m.err != nil {
		return m.err
	}

	// 构建器已关闭(Close)时快速失败，避免获取成功后后台续租因生命周期上下文取消而静默失效
	if m.ctx.Err() != nil {
		return errors.ErrIllegalOperation
	}

	var (
		args    = redis.SetArgs{Mode: "NX", TTL: m.opts.expiration}
		retries int
	)

	for {
		val, err := m.opts.client.SetArgs(ctx, key, version, args).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}

		if val == "OK" {
			return nil
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
		case <-m.ctx.Done():
			// 构建器在等待期间被关闭，停止获取
			ticker.Stop()
			return errors.ErrIllegalOperation
		case <-ticker.C:
			ticker.Stop()
		}
	}
}

// 尝试获取锁
// 仅执行一次 SET NX 写入，不等待也不重试；可通过 expiration 指定固定过期时间，未指定时采用配置的默认过期时间
// @param ctx context.Context 上下文
// @param key string redis键
// @param version string 锁版本标识
// @param expiration ...time.Duration 可选的固定过期时间；为空或小于等于0时采用默认过期时间
// @return @1 error 获取成功返回nil；锁已被他人持有返回errors.ErrIllegalOperation；构建器已关闭返回errors.ErrIllegalOperation
func (m *Maker) tryAcquire(ctx context.Context, key, version string, expiration ...time.Duration) error {
	if m.err != nil {
		return m.err
	}

	// 构建器已关闭(Close)时快速失败，避免获取成功后后台续租因生命周期上下文取消而静默失效
	if m.ctx.Err() != nil {
		return errors.ErrIllegalOperation
	}

	args := redis.SetArgs{Mode: "NX", TTL: m.opts.expiration}

	if len(expiration) > 0 && expiration[0] > 0 {
		args.TTL = expiration[0]
	}

	val, err := m.opts.client.SetArgs(ctx, key, version, args).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	if val != "OK" {
		return errors.ErrIllegalOperation
	}

	return nil
}

// 执行释放锁操作
// 通过 Lua 脚本按版本标识原子地删除锁，仅锁的所有者(版本标识匹配)能释放成功
// @param ctx context.Context 上下文
// @param key string redis键
// @param version string 锁版本标识
// @return @1 error 释放成功返回nil；锁已丢失或所有权已变更返回errors.ErrIllegalOperation
func (m *Maker) release(ctx context.Context, key, version string) error {
	if m.err != nil {
		return m.err
	}

	rst, err := m.releaseScript.Run(ctx, m.opts.client, []string{key}, version).Int()
	if err != nil {
		return err
	}

	if rst != 1 {
		return errors.ErrIllegalOperation
	}

	return nil
}

// 执行续租锁操作
// 通过 Lua 脚本按版本标识原子地刷新锁的过期时间；首次操作失败(瞬时故障)时按指数退避重试，
// 退避睡眠总时长预算控制在锁过期时间的一半以内(见backoffRetries)，尽量避免退避阻塞导致锁过期；
// 退避仍失败则交由续租调度器(locker.go)按短间隔(renewalRetryInterval)继续补偿重试；
// 一旦锁所有权丢失(返回errors.ErrIllegalOperation)则立即终止
// @param ctx context.Context 上下文
// @param key string redis键
// @param version string 锁版本标识
// @return @1 error 续租成功返回nil；锁已丢失或所有权已变更返回errors.ErrIllegalOperation
func (m *Maker) renewal(ctx context.Context, key, version string) error {
	if m.err != nil {
		return m.err
	}

	var (
		keys       = []string{key}
		expiration = m.opts.expiration.Milliseconds()
		renew      = func(ctx context.Context) error {
			rst, err := m.renewalScript.Run(ctx, m.opts.client, keys, version, expiration).Int()
			if err != nil {
				return err
			}

			if rst != 1 {
				return errors.ErrIllegalOperation
			}

			return nil
		}
	)

	err := renew(ctx)
	if err == nil {
		return nil
	}

	if errors.Is(err, errors.ErrIllegalOperation) {
		return err
	}

	retries, baseDelay := backoffRetries(m.opts.expiration)
	if retries == 0 {
		// 过期时间过短、退避预算不足，不再退避重试，交由续租调度器按短间隔(renewalRetryInterval)补偿重试
		return err
	}

	return xcall.Backoff(ctx, func(ctx context.Context, attempt int) (bool, error) {
		err := renew(ctx)
		if err == nil || errors.Is(err, errors.ErrIllegalOperation) {
			return false, err
		}

		return true, err
	}, retries, baseDelay, time.Second)
}

// 计算续租退避的重试次数与初始间隔
// 指数退避(间隔按2倍递增，100ms起步)的总时长被限制在锁过期时间的一半以内，
// 避免退避阻塞续租调度导致锁在故障恢复前过期；最多退避3次，不足一次退避预算时不重试
// @param expiration time.Duration 锁过期时长
// @return @1 int 退避重试次数，最小为0(预算不足以支撑一次退避时不再重试)
// @return @2 time.Duration 退避初始间隔，恒为100ms
func backoffRetries(expiration time.Duration) (int, time.Duration) {
	var (
		delay   = 100 * time.Millisecond
		budget  = expiration / 2
		retries = 0
	)

	for retries < 3 && delay <= budget {
		budget -= delay
		delay *= 2
		retries++
	}

	return retries, 100 * time.Millisecond
}
