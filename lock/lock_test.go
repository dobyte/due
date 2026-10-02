package lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/lock"
)

// fakeLocker records the calls it receives.
type fakeLocker struct {
	acquired bool
	released bool
}

func (l *fakeLocker) Acquire(context.Context) error {
	l.acquired = true

	return nil
}

func (l *fakeLocker) TryAcquire(context.Context, ...time.Duration) error {
	l.acquired = true

	return nil
}

func (l *fakeLocker) Release(context.Context) error {
	l.released = true

	return nil
}

// fakeMaker records the calls it receives and hands out a single fakeLocker.
type fakeMaker struct {
	locker *fakeLocker
	name   string
	closed bool
}

func (m *fakeMaker) Make(name string) lock.Locker {
	m.name = name

	return m.locker
}

func (m *fakeMaker) Close() error {
	m.closed = true

	return nil
}

func TestMake(t *testing.T) {
	maker := &fakeMaker{locker: &fakeLocker{}}
	lock.SetMaker(maker)

	if got := lock.GetMaker(); got != lock.Maker(maker) {
		t.Fatal("expect the maker to be set")
	}

	locker := lock.Make("lockName")
	if locker == nil {
		t.Fatal("expect a locker to be created")
	}
	if maker.name != "lockName" {
		t.Fatalf("invalid lock name, expect: lockName, actual: %s", maker.name)
	}

	ctx := context.Background()

	if err := locker.Acquire(ctx); err != nil {
		t.Fatalf("acquire lock failed, err: %v", err)
	}
	if !maker.locker.acquired {
		t.Fatal("expect the lock to be acquired")
	}

	if err := locker.Release(ctx); err != nil {
		t.Fatalf("release lock failed, err: %v", err)
	}
	if !maker.locker.released {
		t.Fatal("expect the lock to be released")
	}
}

func TestSetMakerClosesPreviousMaker(t *testing.T) {
	previous := &fakeMaker{locker: &fakeLocker{}}
	lock.SetMaker(previous)

	current := &fakeMaker{locker: &fakeLocker{}}
	lock.SetMaker(current)

	if !previous.closed {
		t.Fatal("expect the previous maker to be closed")
	}
	if got := lock.GetMaker(); got != lock.Maker(current) {
		t.Fatal("expect the current maker to be set")
	}
}

func TestSetMakerNilKeepsCurrentMaker(t *testing.T) {
	current := &fakeMaker{locker: &fakeLocker{}}
	lock.SetMaker(current)

	lock.SetMaker(nil)

	if got := lock.GetMaker(); got != lock.Maker(current) {
		t.Fatal("expect a nil maker to be ignored")
	}
	if current.closed {
		t.Fatal("expect the current maker not to be closed")
	}
}

func TestClose(t *testing.T) {
	maker := &fakeMaker{locker: &fakeLocker{}}
	lock.SetMaker(maker)

	if err := lock.Close(); err != nil {
		t.Fatalf("close lock-maker failed, err: %v", err)
	}
	if !maker.closed {
		t.Fatal("expect the maker to be closed")
	}
}
