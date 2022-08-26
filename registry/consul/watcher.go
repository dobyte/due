package consul

import (
	"context"
	"github.com/dobyte/due/registry"
	"sync/atomic"
)

type watcher struct {
	ctx              context.Context
	cancel           context.CancelFunc
	event            chan struct{}
	serviceName      string
	serviceInstances atomic.Value
}

func newWatcher(ctx context.Context, serviceName string) *watcher {
	w := &watcher{}
	w.event = make(chan struct{}, 1)
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.serviceName = serviceName

	return w
}

func (w *watcher) services() []*registry.ServiceInstance {
	return w.serviceInstances.Load().([]*registry.ServiceInstance)
}

func (w *watcher) update(services []*registry.ServiceInstance) {
	w.serviceInstances.Store(services)
	w.event <- struct{}{}
}

// Next 返回服务实例列表
func (w *watcher) Next() (services []*registry.ServiceInstance, err error) {
	select {
	case <-w.ctx.Done():
		err = w.ctx.Err()
	case <-w.event:
		if ss, ok := w.serviceInstances.Load().([]*registry.ServiceInstance); ok {
			services = append(services, ss...)
		}
	}
	return
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	close(w.event)
	return nil
}
