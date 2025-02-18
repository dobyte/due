package redis

import (
	"context"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/go-redis/redis/v8"
	"time"
)

type Maker struct {
	opts          *options
	builtin       bool
	releaseScript *redis.Script
	renewalScript *redis.Script
}

func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.expiration <= 0 {
		o.expiration = xconv.Duration(defaultExpiration)
	}

	m := &Maker{}
	m.opts = o
	m.releaseScript = redis.NewScript(releaseScript)
	m.renewalScript = redis.NewScript(renewalScript)

	if o.client == nil {
		m.builtin = true
		o.client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		})
	}

	return m
}

// Make 制造一个Locker
func (m *Maker) Make(name string) lock.Locker {
	l := &Locker{}
	l.maker = m
	l.version = xuuid.UUID()

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
			if retries > m.opts.acquireMaxRetries {
				return errors.ErrDeadlineExceeded
			}

			retries++
		}

		time.Sleep(m.opts.acquireInterval)
	}
}

// 尝试获取锁
func (m *Maker) tryAcquire(ctx context.Context, key, version string, expiration ...time.Duration) error {
	args := redis.SetArgs{Mode: "NX", TTL: m.opts.expiration}

	if len(expiration) > 0 {
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
func (m *Maker) release(ctx context.Context, key, version string) error {
	rst, err := m.releaseScript.Run(ctx, m.opts.client, []string{key}, version).StringSlice()
	if err != nil {
		return err
	}

	if rst[0] != "OK" {
		return errors.ErrIllegalOperation
	}

	return nil
}

// 执行续租锁操作
func (m *Maker) renewal(ctx context.Context, key, version string) error {
	rst, err := m.renewalScript.Run(ctx, m.opts.client, []string{key}, version, m.opts.expiration.Milliseconds()).StringSlice()
	if err != nil {
		return err
	}

	if rst[0] != "OK" {
		return errors.ErrIllegalOperation
	}

	return nil
}
