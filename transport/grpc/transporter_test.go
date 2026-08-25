package grpc_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/dobyte/due/transport/grpc/v2"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

type mockWatcher struct {
	ch        chan struct{}
	stopCalls atomic.Int32
}

func newMockWatcher() *mockWatcher {
	return &mockWatcher{ch: make(chan struct{})}
}

func (w *mockWatcher) Next() ([]*registry.ServiceInstance, error) {
	<-w.ch
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

type mockDiscovery struct {
	watcher *mockWatcher
}

func (d *mockDiscovery) Watch(_ context.Context, _ string) (registry.Watcher, error) {
	return d.watcher, nil
}

func (d *mockDiscovery) Services(_ context.Context, _ string) ([]*registry.ServiceInstance, error) {
	return nil, nil
}

// TestTransporterClose 验证 Transporter.Close 端到端释放链路
func TestTransporterClose(t *testing.T) {
	w := newMockWatcher()
	tr := grpc.NewTransporter(grpc.WithClientDiscovery(&mockDiscovery{watcher: w}))

	client, err := tr.NewClient("direct://127.0.0.1:8011")
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	_ = client

	if err := tr.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// 监听器通过完整链路被停止
	if n := w.stopCalls.Load(); n != 1 {
		t.Errorf("watcher.Stop should be called once, got %d", n)
	}

	// 关闭后 NewClient 返回 ErrClientClosed
	if _, err := tr.NewClient("direct://127.0.0.1:8011"); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("NewClient after Close should return ErrClientClosed, got %v", err)
	}
}
