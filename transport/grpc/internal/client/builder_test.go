package client

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/connectivity"
)

// mockWatcher simulates a registry watcher.
type mockWatcher struct {
	ch          chan struct{}
	nextEntered atomic.Int32 // Number of times Next has been called, including blocked calls
	nextCalls   atomic.Int32 // Number of times Next has returned
	stopCalls   atomic.Int32 // Number of times Stop has been called
}

func newMockWatcher() *mockWatcher {
	return &mockWatcher{ch: make(chan struct{})}
}

func (w *mockWatcher) Next() ([]*registry.ServiceInstance, error) {
	w.nextEntered.Add(1)
	<-w.ch
	w.nextCalls.Add(1)
	return nil, errors.ErrWatcherStopped
}

func (w *mockWatcher) Stop() error {
	w.stopCalls.Add(1)
	select {
	case <-w.ch:
	default:
		close(w.ch)
	}
	return nil
}

// mockDiscovery simulates a service discovery component.
type mockDiscovery struct {
	watcher *mockWatcher
}

func (d *mockDiscovery) Watch(_ context.Context, _ string) (registry.Watcher, error) {
	return d.watcher, nil
}

func (d *mockDiscovery) Services(_ context.Context, _ string) ([]*registry.ServiceInstance, error) {
	return nil, nil
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

// TestBuilderClose verifies whether the Builder.Close path releases all resources thoroughly.
func TestBuilderClose(t *testing.T) {
	discovery := &mockDiscovery{watcher: newMockWatcher()}
	b := NewBuilder(&Options{Discovery: discovery})
	if b.err != nil {
		t.Fatalf("NewBuilder error: %v", b.err)
	}
	t.Cleanup(func() { _ = b.Close() })

	// Wait for the watch goroutine to start and block in Next.
	waitFor(t, 2*time.Second, func() bool { return discovery.watcher.nextEntered.Load() >= 1 })

	// Open several connections and verify that all of them are released.
	cc1, err := b.Build("direct://127.0.0.1:8011")
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	cc2, err := b.Build("direct://127.0.0.1:8012")
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	// Record the baseline goroutine count (including gRPC internal goroutines).
	before := runtime.NumGoroutine()

	if err := b.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// 1. The watcher is stopped.
	if n := discovery.watcher.stopCalls.Load(); n != 1 {
		t.Errorf("watcher.Stop should be called once, got %d", n)
	}

	// 2. The watch goroutine receives ErrWatcherStopped and exits the blocked Next.
	waitFor(t, 2*time.Second, func() bool { return discovery.watcher.nextCalls.Load() >= 1 })

	// 3. The connection cache is cleared.
	var remain int
	b.connections.Range(func(_, _ any) bool {
		remain++
		return true
	})
	if remain != 0 {
		t.Errorf("connections should be cleared, %d remain", remain)
	}

	// 4. All established connections are closed.
	if state := cc1.GetState(); state != connectivity.Shutdown {
		t.Errorf("connection 1 should be Shutdown, got %v", state)
	}
	if state := cc2.GetState(); state != connectivity.Shutdown {
		t.Errorf("connection 2 should be Shutdown, got %v", state)
	}

	// 5. Close is idempotent.
	if err := b.Close(); err != nil {
		t.Errorf("second Close should return nil, got %v", err)
	}

	// 6. Build returns ErrClientClosed after Close.
	if _, err := b.Build("direct://127.0.0.1:8011"); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("Build after Close should return ErrClientClosed, got %v", err)
	}

	// 7. No goroutine leak (wait for gRPC internal goroutines to exit).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Errorf("goroutine leak detected: %d extra goroutines running", n-before)
	}
}
