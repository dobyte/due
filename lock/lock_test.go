package lock_test

import (
	"context"
	"github.com/dobyte/due/v2/lock"
	"testing"
)

func TestMake(t *testing.T) {
	locker := lock.Make("lockName")

	if err := locker.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	defer locker.Release(context.Background())

}
