package nacos

import (
	"context"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"sync"
	"sync/atomic"
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
	err              error
	ctx              context.Context
	cancel           context.CancelFunc
	registry         *Registry
	serviceName      string
	serviceInstances atomic.Value
	serviceWaitIndex uint64
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
}

func newWatcherMgr(registry *Registry, ctx context.Context, serviceName string) (*watcherMgr, error) {
	services, err := registry.services(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	wm := &watcherMgr{}
	wm.ctx, wm.cancel = context.WithCancel(registry.ctx)
	wm.registry = registry
	wm.serviceName = serviceName
	wm.serviceInstances.Store(services)
	wm.watchers = make(map[int64]*watcher)

	if err = wm.subscribe(); err != nil {
		return nil, err
	}

	return wm, nil
}

func (wm *watcherMgr) subscribe() error {
	return wm.registry.opts.client.Subscribe(&vo.SubscribeParam{
		ServiceName: wm.serviceName,
		Clusters:    []string{wm.registry.opts.clusterName},
		GroupName:   wm.registry.opts.groupName,
		SubscribeCallback: func(instances []model.Instance, err error) {
			if err != nil {
				log.Warnf("%s subscribe callback failed: %v", wm.serviceName, err)
				return
			}

			services, err := parseInstances(instances)
			if err != nil {
				log.Warnf("%s instances parse failed: %v", wm.serviceName, err)
				return
			}

			wm.serviceInstances.Store(services)
			wm.broadcast(services)
		},
	})
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

func (wm *watcherMgr) broadcast(services []*registry.ServiceInstance) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	for _, w := range wm.watchers {
		w.notify(services)
	}
}

func (wm *watcherMgr) services() []*registry.ServiceInstance {
	return wm.serviceInstances.Load().([]*registry.ServiceInstance)
}
