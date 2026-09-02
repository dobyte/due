package redis_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/lock/redis/v2"
	"github.com/dobyte/due/v2/errors"
	goredis "github.com/redis/go-redis/v9"
)

// requireRedis 探测本地 redis 服务是否可用，不可用时跳过测试
func requireRedis(t *testing.T) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 200*time.Millisecond)
	if err != nil {
		t.Skipf("local redis is not available: %v", err)
	}

	conn.Close()
}

func TestLocker_Acquire(t *testing.T) {
	requireRedis(t)

	var (
		ctx    = context.Background()
		maker  = redis.NewMaker()
		locker = maker.Make("lockName")
		other  = maker.Make("lockName")
	)

	t.Cleanup(func() { _ = maker.Close() })

	if err := locker.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	// 锁被持有时，其他 Locker 无法获取
	if err := other.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got: %v", err)
	}

	if err := locker.Release(ctx); err != nil {
		t.Fatal(err)
	}

	// 释放后，其他 Locker 可以获取
	if err := other.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	defer other.Release(ctx)
}

func TestLocker_Parallel_Acquire(t *testing.T) {
	requireRedis(t)

	var (
		wg      sync.WaitGroup
		ctx     = context.Background()
		maker   = redis.NewMaker()
		holders atomic.Int32
	)

	t.Cleanup(func() { _ = maker.Close() })

	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			// 每个竞争者持有独立的 Locker(独立version)竞争同一把锁
			locker := maker.Make("lockName")

			if err := locker.Acquire(ctx); err != nil {
				t.Errorf("%d acquire lock failed: %v", i, err)
				return
			}

			defer func() {
				if err := locker.Release(ctx); err != nil {
					t.Errorf("%d release lock failed: %v", i, err)
				}
			}()

			// 任意时刻只允许一个持有者进入临界区
			if n := holders.Add(1); n != 1 {
				t.Errorf("%d lock is not exclusive, concurrent holders: %d", i, n)
			}

			t.Logf("%d do some things", i)

			time.Sleep(100 * time.Millisecond)

			holders.Add(-1)
		}(i)
	}

	wg.Wait()
}

func TestLocker_Renewal(t *testing.T) {
	requireRedis(t)

	var (
		ctx    = context.Background()
		maker  = redis.NewMaker(redis.WithExpiration(500 * time.Millisecond))
		locker = maker.Make("lockRenewal")
		other  = maker.Make("lockRenewal")
	)

	t.Cleanup(func() { _ = maker.Close() })

	if err := locker.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	defer locker.Release(ctx)

	// 等待时长超过默认过期时间；若后台续租未生效，锁早已过期、他人应能获取
	time.Sleep(1200 * time.Millisecond)

	if err := other.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation since the lock is renewed, got: %v", err)
	}
}

func TestLocker_Expired_Release(t *testing.T) {
	requireRedis(t)

	var (
		ctx    = context.Background()
		maker  = redis.NewMaker()
		locker = maker.Make("lockExpired")
	)

	t.Cleanup(func() { _ = maker.Close() })

	// 以固定过期时间获取锁，不开启后台续租
	if err := locker.TryAcquire(ctx, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// 等待锁自然过期后再释放，应能感知锁已丢失
	time.Sleep(400 * time.Millisecond)

	if err := locker.Release(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation when release an expired lock, got: %v", err)
	}
}

func TestMaker_Closed(t *testing.T) {
	requireRedis(t)

	var (
		ctx = context.Background()
		// 外部客户端，生命周期不由 Maker 管理，用于验证 Close 后获取锁能快速失败
		// (内建客户端场景会因连接池关闭而报错，外部客户端需显式拦截)
		client = goredis.NewUniversalClient(&goredis.UniversalOptions{Addrs: []string{"127.0.0.1:6379"}})
		maker  = redis.NewMaker(redis.WithClient(client))
		locker = maker.Make("lockClosed")
	)

	t.Cleanup(func() { _ = client.Close() })

	if err := maker.Close(); err != nil {
		t.Fatal(err)
	}

	// Close 后获取锁应快速失败，而非获取成功但后台续租因生命周期上下文取消而静默失效
	if err := locker.TryAcquire(ctx); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation after maker closed, got: %v", err)
	}
}
