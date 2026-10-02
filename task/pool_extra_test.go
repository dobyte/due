package task

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
)

// fakePool is a Pool implementation that lets a test force specific results without a real
// goroutine pool.
type fakePool struct {
	addErr   error
	released atomic.Bool
}

// AddTask returns the configured error without running task.
func (p *fakePool) AddTask(task func()) error { return p.addErr }

// Release marks the pool as released.
func (p *fakePool) Release() { p.released.Store(true) }

// TestNewPoolWithOptions verifies that the pool options are applied to the underlying ants pool.
func TestNewPoolWithOptions(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{name: "default size", size: defaultSize},
		{name: "custom size", size: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPool(WithSize(tt.size), WithNonblocking(false), WithDisablePurge(false))
			defer p.Release()

			if got := p.pool.Cap(); got != tt.size {
				t.Errorf("invalid pool size, expect: %d, actual: %d", tt.size, got)
			}
		})
	}
}

// TestDefaultOptions verifies the built-in defaults returned when no configuration is present.
func TestDefaultOptions(t *testing.T) {
	o := defaultOptions()

	if o.size != defaultSize {
		t.Errorf("invalid default size, expect: %d, actual: %d", defaultSize, o.size)
	}
	if o.nonblocking != defaultNonblocking {
		t.Errorf("invalid default nonblocking, expect: %v, actual: %v", defaultNonblocking, o.nonblocking)
	}
	if o.disablePurge != defaultDisablePurge {
		t.Errorf("invalid default disablePurge, expect: %v, actual: %v", defaultDisablePurge, o.disablePurge)
	}
}

// TestDefaultPoolAddTask verifies that submitted tasks are executed by the pool.
func TestDefaultPoolAddTask(t *testing.T) {
	p := NewPool(WithSize(4), WithNonblocking(false), WithDisablePurge(true))
	defer p.Release()

	const total = 20

	var (
		wg    sync.WaitGroup
		count atomic.Int64
	)

	wg.Add(total)
	for range total {
		if err := p.AddTask(func() {
			count.Add(1)
			wg.Done()
		}); err != nil {
			t.Fatalf("submit task failed, err: %v", err)
		}
	}

	wg.Wait()

	if got := count.Load(); got != total {
		t.Errorf("invalid executed task count, expect: %d, actual: %d", total, got)
	}
}

// TestDefaultPoolNonblockingOverflow verifies that a submission fails immediately when a
// non-blocking pool is full.
func TestDefaultPoolNonblockingOverflow(t *testing.T) {
	p := NewPool(WithSize(1), WithNonblocking(true), WithDisablePurge(true))
	defer p.Release()

	var (
		started = make(chan struct{})
		block   = make(chan struct{})
	)

	if err := p.AddTask(func() {
		close(started)
		<-block
	}); err != nil {
		t.Fatalf("submit blocking task failed, err: %v", err)
	}

	<-started

	if err := p.AddTask(func() {}); err == nil {
		t.Error("expect an error when submitting to a full non-blocking pool")
	}

	close(block)
}

// TestSetPoolAndGetPool verifies that the global pool can be replaced and released.
func TestSetPoolAndGetPool(t *testing.T) {
	defer SetPool(NewPool())

	fake := &fakePool{}
	SetPool(fake)

	if got := GetPool(); got != Pool(fake) {
		t.Error("expect the global pool to be replaced")
	}
}

// TestSetPoolReleasesPrevious verifies that replacing the global pool releases the previous one.
func TestSetPoolReleasesPrevious(t *testing.T) {
	defer SetPool(NewPool())

	prev := &fakePool{}
	SetPool(prev)
	SetPool(&fakePool{})

	if !prev.released.Load() {
		t.Error("expect the previous global pool to be released")
	}
}

// TestAddDegradesToGoroutine verifies that Add still runs the task when the global pool rejects
// the submission.
func TestAddDegradesToGoroutine(t *testing.T) {
	defer SetPool(NewPool())

	SetPool(&fakePool{addErr: errors.New("pool is full")})

	done := make(chan struct{})
	Add(func() { close(done) })

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expect the task to run in a degraded goroutine")
	}
}

// TestAddTaskDegradesToGoroutine verifies that the deprecated AddTask behaves like Add.
func TestAddTaskDegradesToGoroutine(t *testing.T) {
	defer SetPool(NewPool())

	SetPool(&fakePool{addErr: errors.New("pool is full")})

	done := make(chan struct{})
	AddTask(func() { close(done) })

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expect the task to run in a degraded goroutine")
	}
}

// TestAddWithoutGlobalPool verifies that Add runs the task in its own goroutine when the global
// pool is unavailable.
func TestAddWithoutGlobalPool(t *testing.T) {
	SetPool(nil)
	defer SetPool(NewPool())

	done := make(chan struct{})
	Add(func() { close(done) })

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expect the task to run when the global pool is missing")
	}
}

// TestAddTaskWithoutGlobalPool verifies that the deprecated AddTask runs the task in its own
// goroutine when the global pool is unavailable.
func TestAddTaskWithoutGlobalPool(t *testing.T) {
	SetPool(nil)
	defer SetPool(NewPool())

	done := make(chan struct{})
	AddTask(func() { close(done) })

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expect the task to run when the global pool is missing")
	}
}

// TestRelease verifies that releasing the global pool is safe and can be repeated.
func TestRelease(t *testing.T) {
	defer SetPool(NewPool())

	SetPool(NewPool(WithSize(1)))

	Release()
	Release()
}

// TestLoggerPrintf verifies that the log adapter forwards formatted messages.
func TestLoggerPrintf(t *testing.T) {
	(&logger{}).Printf("task logger print: %d", 1)
}
