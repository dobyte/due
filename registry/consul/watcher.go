package consul

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/registry"
)

type watcher struct {
	idx        int64
	state      atomic.Bool
	ctx        context.Context
	cancel     context.CancelFunc
	watcherMgr *watcherMgr
	chWatch    chan []*registry.ServiceInstance
}

func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(wm.ctx)
	w.idx = idx
	w.watcherMgr = wm
	w.chWatch = make(chan []*registry.ServiceInstance, 16)

	return w
}

func (w *watcher) notify(services []*registry.ServiceInstance) {
	if w.state.Load() {
		w.chWatch <- services
	}
}

// Next 返回服务实例列表
func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	if w.state.CompareAndSwap(false, true) {
		return w.watcherMgr.services(), nil
	}

	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case services, ok := <-w.chWatch:
		if !ok {
			if err := w.ctx.Err(); err != nil {
				return nil, err
			}
		}

		return services, nil
	}
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	close(w.chWatch)
	return w.watcherMgr.recycle(w.idx)
}

type watcherMgr struct {
	ctx              context.Context
	cancel           context.CancelFunc
	registry         *Registry
	serviceName      string
	serviceInstances *atomic.Value
	serviceWaitIndex uint64
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
}

func newWatcherMgr(registry *Registry, ctx context.Context, serviceName string) (*watcherMgr, error) {
	services, index, err := registry.services(ctx, serviceName, 0, true)
	if err != nil {
		return nil, err
	}

	wm := &watcherMgr{}
	wm.ctx, wm.cancel = context.WithCancel(registry.ctx)
	wm.registry = registry
	wm.serviceName = serviceName
	wm.serviceInstances = &atomic.Value{}
	wm.serviceWaitIndex = index
	wm.serviceInstances.Store(services)
	wm.watchers = make(map[int64]*watcher)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-wm.ctx.Done():
				return
			case <-ticker.C:
				tctx, tcancel := context.WithTimeout(ctx, 120*time.Second)
				services, index, err = wm.registry.services(tctx, wm.serviceName, wm.serviceWaitIndex, true)
				tcancel()
				if err != nil {
					time.Sleep(time.Second)
					continue
				}

				if index != wm.serviceWaitIndex {
					wm.serviceWaitIndex = index
					wm.serviceInstances.Store(services)
					wm.broadcast()
				}
			}
		}
	}()

	return wm, nil
}

func (wm *watcherMgr) fork() registry.Watcher {
	wm.rw.Lock()
	defer wm.rw.Unlock()

	w := newWatcher(wm, wm.idx.Add(1))
	wm.watchers[w.idx] = w

	return w
}

func (wm *watcherMgr) recycle(idx int64) error {
	wm.rw.Lock()
	defer wm.rw.Unlock()

	delete(wm.watchers, idx)

	if len(wm.watchers) == 0 {
		wm.cancel()
		wm.registry.watchers.Delete(wm.serviceName)
	}

	return nil
}

func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	services := wm.services()
	for _, w := range wm.watchers {
		w.notify(services)
	}
}

func (wm *watcherMgr) services() []*registry.ServiceInstance {
	return wm.serviceInstances.Load().([]*registry.ServiceInstance)
}
