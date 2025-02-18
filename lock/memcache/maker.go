package memcache

import (
	"context"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/dobyte/due/v2/utils/xuuid"
	"time"
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

	m := &Maker{}
	m.opts = o

	if o.client == nil {
		m.builtin = true
		o.client = memcache.New(o.addrs...)
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
		err     error
		retries int
		item    = &memcache.Item{
			Key:        key,
			Value:      xconv.Bytes(version),
			Expiration: int32(m.opts.expiration.Seconds()),
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
	item := &memcache.Item{Key: key, Value: xconv.Bytes(version)}

	if len(expiration) > 0 && expiration[0] > 0 {
		item.Expiration = int32(expiration[0].Seconds())
	} else {
		item.Expiration = int32(m.opts.expiration.Seconds())
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
	return m.swap(ctx, key, version, int32(xtime.Now().AddDate(-1, 0, 0).Unix()))
}

// 执行续租锁操作
func (m *Maker) renewal(ctx context.Context, key, version string) error {
	return m.swap(ctx, key, version, int32(m.opts.expiration.Seconds()))
}

// 执行替换操作
func (m *Maker) swap(ctx context.Context, key, version string, expiration int32) error {
	item, err := m.opts.client.Get(key)
	if err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			return errors.ErrIllegalOperation
		}

		return err
	}

	if xconv.String(item.Value) != version {
		return errors.ErrIllegalOperation
	}

	item.Expiration = expiration

	if err = m.opts.client.CompareAndSwap(item); err != nil {
		if errors.Is(err, memcache.ErrNotStored) {
			return errors.ErrIllegalOperation
		}

		return err
	}

	return nil
}

func (m *Maker) Get(key string) (*memcache.Item, error) {
	return m.opts.client.Get(key)
}
