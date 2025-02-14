package redis

import (
	"context"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/go-redis/redis/v8"
	"time"
)

type Maker struct {
	opts          *options
	releaseScript *redis.Script
	renewalScript *redis.Script
}

func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.client == nil {
		o.client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		})
	}

	m := &Maker{}
	m.opts = o
	m.releaseScript = redis.NewScript(releaseScript)
	m.renewalScript = redis.NewScript(renewalScript)

	return m
}

// Make 制造一个Locker
func (m *Maker) Make(name string) lock.Locker {
	return &Locker{
		maker:   m,
		name:    name,
		version: xuuid.UUID(),
	}
}

// 执行获取锁操作
func (m *Maker) acquire(ctx context.Context, name, version string) error {
	var retries int

	for {
		val, err := m.opts.client.SetArgs(ctx, name, version, redis.SetArgs{
			Mode: "NX",
			TTL:  m.opts.expiration,
		}).Result()
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

// 执行释放锁操作
func (m *Maker) release(ctx context.Context, name, version string) error {
	rst, err := m.releaseScript.Run(ctx, m.opts.client, []string{name}, version).StringSlice()
	if err != nil {
		return err
	}

	if rst[0] != "OK" {
		return errors.ErrIllegalOperation
	}

	return nil
}

// 执行续租锁操作
func (m *Maker) renewal(ctx context.Context, name, version string) error {
	rst, err := m.renewalScript.Run(ctx, m.opts.client, []string{name}, version, m.opts.expiration).StringSlice()
	if err != nil {
		return err
	}

	if rst[0] != "OK" {
		return errors.ErrIllegalOperation
	}

	return nil
}
