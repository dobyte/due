package node

import (
	"testing"
	"time"
)

// TestTimerStop verifies the timer stop paths.
func TestTimerStop(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var timer *Timer

		if timer.Stop() {
			t.Fatal("stopping a nil timer should report false")
		}
	})

	t.Run("stopped before firing", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		if !n.doAddWait() {
			t.Fatal("expect the wait count to be registered")
		}

		timer := &Timer{node: n, timer: time.AfterFunc(time.Hour, func() {})}
		if !timer.Stop() {
			t.Fatal("expect the timer to be stopped")
		}
		if got := n.counter.Load(); got != 0 {
			t.Fatalf("the wait count should be decremented, got %d", got)
		}
	})

	t.Run("stopped without node", func(t *testing.T) {
		timer := &Timer{timer: time.AfterFunc(time.Hour, func() {})}
		if !timer.Stop() {
			t.Fatal("expect the timer to be stopped")
		}
	})

	t.Run("stopped after firing", func(t *testing.T) {
		fired := make(chan struct{})
		timer := &Timer{timer: time.AfterFunc(time.Millisecond, func() { close(fired) })}
		<-fired
		time.Sleep(10 * time.Millisecond)

		if timer.Stop() {
			t.Fatal("stopping an already fired timer should report false")
		}
	})
}

// TestTimerFromProxy verifies that a proxy timer decrements the wait count when stopped.
func TestTimerFromProxy(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	timer, err := n.Proxy().AfterFunc(time.Hour, func() {})
	if err != nil {
		t.Fatalf("after func failed: %v", err)
	}
	if got := n.counter.Load(); got != 1 {
		t.Fatalf("expect the wait count to be 1, got %d", got)
	}
	if !timer.Stop() {
		t.Fatal("expect the timer to be stopped")
	}
	if got := n.counter.Load(); got != 0 {
		t.Fatalf("expect the wait count to be 0, got %d", got)
	}
}
