package nacos

import (
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

const (
	stateInitial int32 = iota
	stateRunning
	stateStopped
)

type watcher struct {
	idx     int64
	wm      *watcherMgr
	state   atomic.Int32
	mu      sync.Mutex
	chWatch chan []*registry.ServiceInstance
}

func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 16)

	return w
}

func (w *watcher) notify(services []*registry.ServiceInstance) {
	if w.state.Load() != stateRunning {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state.Load() != stateRunning {
		return
	}

	w.flush()

	w.chWatch <- services
}

func (w *watcher) flush() {
	for {
		select {
		case <-w.chWatch:
		default:
			return
		}
	}
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	if w.state.CompareAndSwap(stateInitial, stateRunning) {
		return w.wm.services()
	}

	services, ok := <-w.chWatch
	if !ok {
		return nil, errors.ErrWatcherStopped
	}

	return services, nil
}

func (w *watcher) Stop() error {
	if w.state.Swap(stateStopped) == stateStopped {
		return errors.ErrIllegalOperation
	}

	w.mu.Lock()
	close(w.chWatch)
	w.mu.Unlock()

	w.wm.recycle(w.idx)

	return nil
}

type watcherMgr struct {
	registry         *Registry
	serviceName      string
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
	stopped          atomic.Bool
	err              error
	serviceInstances []*registry.ServiceInstance
}

func newWatcherMgr(registry *Registry, serviceName string, services []*registry.ServiceInstance) (*watcherMgr, error) {
	wm := &watcherMgr{}
	wm.registry = registry
	wm.serviceName = serviceName
	wm.watchers = make(map[int64]*watcher)
	wm.serviceInstances = services

	if err := wm.subscribe(); err != nil {
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

			wm.rw.Lock()
			wm.serviceInstances = services
			wm.rw.Unlock()

			wm.broadcast()
		},
	})
}

func (wm *watcherMgr) fork() registry.Watcher {
	wm.rw.Lock()
	defer wm.rw.Unlock()

	if wm.stopped.Load() {
		return nil
	}

	w := newWatcher(wm, wm.idx.Add(1))
	wm.watchers[w.idx] = w

	return w
}

func (wm *watcherMgr) recycle(idx int64) {
	wm.rw.Lock()
	delete(wm.watchers, idx)
	if len(wm.watchers) != 0 {
		wm.rw.Unlock()
		return
	}

	if !wm.stopped.CompareAndSwap(false, true) {
		wm.rw.Unlock()
		return
	}

	wm.registry.watchers.Delete(wm.serviceName)
	wm.rw.Unlock()

	wm.unsubscribe()
}

func (wm *watcherMgr) stop() {
	wm.rw.Lock()
	if !wm.stopped.CompareAndSwap(false, true) {
		wm.rw.Unlock()
		return
	}

	wm.registry.watchers.Delete(wm.serviceName)
	watchers := wm.loadWatchers()
	wm.rw.Unlock()

	for _, w := range watchers {
		w.Stop()
	}

	wm.unsubscribe()
}

func (wm *watcherMgr) unsubscribe() {
	if err := wm.registry.opts.client.Unsubscribe(&vo.SubscribeParam{
		ServiceName: wm.serviceName,
		Clusters:    []string{wm.registry.opts.clusterName},
		GroupName:   wm.registry.opts.groupName,
	}); err != nil {
		log.Warnf("%s unsubscribe failed: %v", wm.serviceName, err)
	}
}

func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

func (wm *watcherMgr) loadWatchers() []*watcher {
	watchers := make([]*watcher, 0, len(wm.watchers))

	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	return watchers
}

func (wm *watcherMgr) loadServices() []*registry.ServiceInstance {
	services := make([]*registry.ServiceInstance, 0, len(wm.serviceInstances))

	for _, ins := range wm.serviceInstances {
		services = append(services, ins)
	}

	return services
}

func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	if wm.err != nil {
		return nil, wm.err
	}

	return wm.loadServices(), nil
}
