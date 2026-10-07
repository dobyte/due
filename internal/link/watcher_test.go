package link

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

// terminalRegistryWatcher returns a snapshot followed by a permanent error.
type terminalRegistryWatcher struct {
	services  []*registry.ServiceInstance
	err       error
	nextCalls atomic.Int32
	stopCalls atomic.Int32
	stopped   chan struct{}
}

func (w *terminalRegistryWatcher) Next() ([]*registry.ServiceInstance, error) {
	if w.nextCalls.Add(1) == 1 {
		return w.services, nil
	}
	return nil, w.err
}

func (w *terminalRegistryWatcher) Stop() error {
	if w.stopCalls.Add(1) == 1 {
		close(w.stopped)
	}
	return nil
}

func TestLinkerWatchClusterInstanceStopsOnTerminalError(t *testing.T) {
	linkers := []struct {
		name    string
		service *registry.ServiceInstance
		new     func(context.Context, *Options) (func(), func(string) bool)
	}{
		{
			name:    "node",
			service: nodeService("node-1", "127.0.0.1:1"),
			new: func(ctx context.Context, opts *Options) (func(), func(string) bool) {
				linker := NewNodeLinker(ctx, opts)
				return linker.WatchClusterInstance, linker.HasNode
			},
		},
		{
			name:    "gate",
			service: gateService("gate-1", "127.0.0.1:1"),
			new: func(ctx context.Context, opts *Options) (func(), func(string) bool) {
				linker := NewGateLinker(ctx, opts)
				return linker.WatchClusterInstance, linker.HasGate
			},
		},
	}
	errorsToReturn := []struct {
		name string
		err  error
	}{
		{name: "watcher stopped", err: errors.ErrWatcherStopped},
		{name: "wrapped watcher stopped", err: fmt.Errorf("registry watch failed: %w", errors.ErrWatcherStopped)},
		{name: "context canceled", err: context.Canceled},
		{name: "wrapped context canceled", err: fmt.Errorf("registry watch failed: %w", context.Canceled)},
	}

	for _, linker := range linkers {
		t.Run(linker.name, func(t *testing.T) {
			for _, terminalError := range errorsToReturn {
				t.Run(terminalError.name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					watcher := &terminalRegistryWatcher{
						services: []*registry.ServiceInstance{linker.service},
						err:      terminalError.err,
						stopped:  make(chan struct{}),
					}
					t.Cleanup(func() {
						cancel()
						select {
						case <-watcher.stopped:
						case <-time.After(time.Second):
							t.Error("watcher was not stopped during cleanup")
						}
					})

					opts := baseLinkOptions()
					opts.Registry = &registryStub{watchFn: func(context.Context, string) (registry.Watcher, error) {
						return watcher, nil
					}}
					start, hasService := linker.new(ctx, opts)
					start()

					select {
					case <-watcher.stopped:
					case <-time.After(time.Second):
						t.Fatal("watcher kept polling after a terminal error")
					}

					if got := watcher.nextCalls.Load(); got != 2 {
						t.Errorf("Next called %d times, want one snapshot and one terminal error", got)
					}
					if got := watcher.stopCalls.Load(); got != 1 {
						t.Errorf("Stop called %d times, want 1", got)
					}
					if !hasService(linker.service.ID) {
						t.Error("the last service snapshot was lost")
					}
				})
			}
		})
	}
}
