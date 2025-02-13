package redis

import (
	"context"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/lock"
	"github.com/go-redis/redis/v8"
)

type Maker struct {
	redis         redis.UniversalClient
	releaseScript *redis.Script
}

func NewMaker() *Maker {

}

// Make 制造一个Locker
func (m *Maker) Make(name string) lock.Locker {
	l := &Locker{
		maker: m,
		name:  name,
	}

	return l
}

// 执行释放锁操作
func (m *Maker) release(ctx context.Context, name, version string) error {
	rst, err := m.releaseScript.Run(ctx, m.redis, []string{name}, version).StringSlice()
	if err != nil {
		return err
	}

	if rst[0] != "OK" {
		return errors.ErrIllegalOperation
	}

	return nil
}
