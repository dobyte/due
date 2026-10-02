package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
)

// TestGroupWaitWithoutError verifies that all tasks run and Wait reports no error.
func TestGroupWaitWithoutError(t *testing.T) {
	var count atomic.Int64

	g, _ := WithContext(context.Background())
	for range 10 {
		g.Go(func() error {
			count.Add(1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Errorf("expect no error, actual: %v", err)
	}
	if got := count.Load(); got != 10 {
		t.Errorf("invalid executed task count, expect: 10, actual: %d", got)
	}
}

// TestGroupReturnsFirstError verifies that Wait reports the first error and cancels the context.
func TestGroupReturnsFirstError(t *testing.T) {
	want := errors.New("first error")

	g, ctx := WithContext(context.Background())

	g.Go(func() error {
		<-ctx.Done()
		return nil
	})
	g.Go(func() error { return want })
	g.Go(func() error {
		<-ctx.Done()
		return nil
	})

	if err := g.Wait(); !errors.Is(err, want) {
		t.Errorf("expect the first error, expect: %v, actual: %v", want, err)
	}
	if cause := context.Cause(ctx); !errors.Is(cause, want) {
		t.Errorf("expect the context to be canceled with the first error, actual: %v", cause)
	}
}

// TestGroupSetLimit verifies that the concurrency limit is honored by Go.
func TestGroupSetLimit(t *testing.T) {
	g, _ := WithContext(context.Background())
	g.SetLimit(2)

	var (
		running atomic.Int64
		peak    atomic.Int64
	)

	for range 8 {
		g.Go(func() error {
			cur := running.Add(1)
			for {
				old := peak.Load()
				if cur <= old || peak.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Errorf("expect no error, actual: %v", err)
	}
	if got := peak.Load(); got > 2 {
		t.Errorf("expect the peak concurrency not to exceed 2, actual: %d", got)
	}
}

// TestGroupTryGoLimit verifies that TryGo reports false once the concurrency limit is reached.
func TestGroupTryGoLimit(t *testing.T) {
	g, _ := WithContext(context.Background())
	g.SetLimit(1)

	var (
		started = make(chan struct{})
		release = make(chan struct{})
	)

	if !g.TryGo(func() error {
		close(started)
		<-release
		return nil
	}) {
		t.Fatal("expect the first TryGo to succeed")
	}

	<-started

	if g.TryGo(func() error { return nil }) {
		t.Error("expect the second TryGo to fail when the limit is reached")
	}

	close(release)

	if err := g.Wait(); err != nil {
		t.Errorf("expect no error, actual: %v", err)
	}
}

// TestGroupSetLimitWithoutLimit verifies that a negative limit disables the concurrency limit.
func TestGroupSetLimitWithoutLimit(t *testing.T) {
	g, _ := WithContext(context.Background())
	g.SetLimit(-1)

	if !g.TryGo(func() error { return nil }) {
		t.Error("expect TryGo to succeed when the concurrency limit is disabled")
	}
	if g.sem != nil {
		t.Error("expect the semaphore to be cleared for a negative limit")
	}

	if err := g.Wait(); err != nil {
		t.Errorf("expect no error, actual: %v", err)
	}
}

// TestGroupModifyLimitWhileActive verifies that changing the limit while tasks are running
// panics.
func TestGroupModifyLimitWhileActive(t *testing.T) {
	g, _ := WithContext(context.Background())
	g.SetLimit(1)

	var (
		started = make(chan struct{})
		release = make(chan struct{})
	)

	g.Go(func() error {
		close(started)
		<-release
		return nil
	})

	<-started

	func() {
		defer func() {
			if recover() == nil {
				t.Error("expect a panic when modifying the limit while tasks are active")
			}
		}()

		g.SetLimit(2)
	}()

	close(release)

	if err := g.Wait(); err != nil {
		t.Errorf("expect no error, actual: %v", err)
	}
}

// TestGroupWithoutCancel verifies that Wait works on a Group built without a context.
func TestGroupWithoutCancel(t *testing.T) {
	g := &Group{}

	var count atomic.Int64
	g.Go(func() error {
		count.Add(1)
		return nil
	})

	if err := g.Wait(); err != nil {
		t.Errorf("expect no error, actual: %v", err)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("invalid executed task count, expect: 1, actual: %d", got)
	}
}
