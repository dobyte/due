package lock_test

import (
	"github.com/dobyte/due/v2/lock"
	"testing"
)

func TestMake(t *testing.T) {
	locker := lock.Make("lockName")

	if err := locker.Acquire(); err != nil {
		t.Fatal(err)
	}

	defer locker.Release()

}
