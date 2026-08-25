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

// mockWatcher 模拟注册中心监听器
type mockWatcher struct {
	ch          chan struct{}
	nextEntered atomic.Int32 // Next 被调用次数（含阻塞中）
	nextCalls   atomic.Int32 // Next 已返回次数
	stopCalls   atomic.Int32 // Stop 被调用次数
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

// mockDiscovery 模拟服务发现组件
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

// TestBuilderClose 验证 Builder.Close 链路是否彻底释放全部资源
func TestBuilderClose(t *testing.T) {
	discovery := &mockDiscovery{watcher: newMockWatcher()}
	b := NewBuilder(&Options{Discovery: discovery})
	if b.err != nil {
		t.Fatalf("NewBuilder error: %v", b.err)
	}
	t.Cleanup(func() { _ = b.Close() })

	// 等待 watch 协程启动并阻塞在 Next()
	waitFor(t, 2*time.Second, func() bool { return discovery.watcher.nextEntered.Load() >= 1 })

	// 建立多个连接，验证全部被释放
	cc1, err := b.Build("direct://127.0.0.1:8011")
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	cc2, err := b.Build("direct://127.0.0.1:8012")
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	// 记录基准协程数（含 grpc 内部协程）
	before := runtime.NumGoroutine()

	if err := b.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// 1. 监听器被停止
	if n := discovery.watcher.stopCalls.Load(); n != 1 {
		t.Errorf("watcher.Stop should be called once, got %d", n)
	}

	// 2. watch 协程收到 ErrWatcherStopped 并退出阻塞的 Next()
	waitFor(t, 2*time.Second, func() bool { return discovery.watcher.nextCalls.Load() >= 1 })

	// 3. 连接缓存已清空
	var remain int
	b.connections.Range(func(_, _ any) bool {
		remain++
		return true
	})
	if remain != 0 {
		t.Errorf("connections should be cleared, %d remain", remain)
	}

	// 4. 已建立的连接全部关闭
	if state := cc1.GetState(); state != connectivity.Shutdown {
		t.Errorf("connection 1 should be Shutdown, got %v", state)
	}
	if state := cc2.GetState(); state != connectivity.Shutdown {
		t.Errorf("connection 2 should be Shutdown, got %v", state)
	}

	// 5. Close 幂等
	if err := b.Close(); err != nil {
		t.Errorf("second Close should return nil, got %v", err)
	}

	// 6. 关闭后 Build 返回 ErrClientClosed
	if _, err := b.Build("direct://127.0.0.1:8011"); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("Build after Close should return ErrClientClosed, got %v", err)
	}

	// 7. 无协程泄漏（等待 grpc 内部协程退出）
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
