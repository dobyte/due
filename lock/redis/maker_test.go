package redis_test

import (
	"context"
	"github.com/dobyte/due/lock/redis/v2"
	"testing"
	"time"
)

func TestMaker_Make(t *testing.T) {
	maker := redis.NewMaker()

	locker := maker.Make("lockName")

	if err := locker.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	defer locker.Release(context.Background())

	panic("abc")

	time.Sleep(20 * time.Second)
}
