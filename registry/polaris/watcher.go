package polaris

import (
	"maps"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// Watcher running state constants.
const (
	stateInitial int32 = iota
	stateRunning
	stateStopped
)

// watcher is a service instance watcher.
type watcher struct {
	idx     int64
	wm      *watcherMgr
	state   atomic.Int32
	mu      sync.Mutex
	chWatch chan []*registry.ServiceInstance
}

// newWatcher creates a service instance watcher.
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 16)

	return w
}

// notify notifies the watcher of service instance updates.
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

// flush empties the data in the channel.
func (w *watcher) flush() {
	for {
		select {
		case <-w.chWatch:
		default:
			return
		}
	}
}

// latest returns the latest service instances.
func (w *watcher) latest() ([]*registry.ServiceInstance, error) {
	var (
		exist     bool
		instances []*registry.ServiceInstance
	)

	for {
		select {
		case services, ok := <-w.chWatch:
			if !ok {
				if exist {
					return instances, nil
				}

				return nil, errors.ErrWatcherStopped
			}

			exist, instances = true, services
		default:
			if exist {
				return instances, nil
			} else {
				return w.wm.services()
			}
		}
	}
}

// Next returns the list of service instances.
func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	if w.state.CompareAndSwap(stateInitial, stateRunning) {
		return w.latest()
	}

	services, ok := <-w.chWatch
	if !ok {
		return nil, errors.ErrWatcherStopped
	}

	return services, nil
}

// Stop stops watching service instance updates.
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

// watcherMgr is a service instance watcher manager.
type watcherMgr struct {
	registry         *Registry
	serviceName      string
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
	stopped          atomic.Bool
	serviceInstances []*registry.ServiceInstance
	resp             *model.WatchAllInstancesResponse
}

// newWatcherMgr creates a service instance watcher manager.
func newWatcherMgr(registry *Registry, serviceName string) *watcherMgr {
	wm := &watcherMgr{}
	wm.registry = registry
	wm.serviceName = serviceName
	wm.watchers = make(map[int64]*watcher)

	return wm
}

// init initializes the service instance watch.
func (wm *watcherMgr) init() error {
	if err := wm.subscribe(); err != nil {
		wm.unsubscribe()
		return err
	}

	// After a successful subscription, initialize the service instances with the snapshot taken at
	// subscription time so that changes made before the subscription are not missed.
	services, err := parseInstances(wm.resp.InstancesResponse().GetInstances())
	if err != nil {
		wm.unsubscribe()
		return err
	}

	wm.rw.Lock()
	wm.serviceInstances = services
	wm.rw.Unlock()

	return nil
}

// subscribe subscribes to service instance changes.
func (wm *watcherMgr) subscribe() error {
	req := &api.WatchAllInstancesRequest{}
	req.Namespace = wm.registry.opts.namespace
	req.Service = wm.serviceName
	req.WatchMode = model.WatchModeNotify
	req.InstancesListener = wm

	resp, err := wm.registry.consumer.WatchAllInstances(req)
	if err != nil {
		return err
	}

	wm.resp = resp

	return nil
}

// OnInstancesUpdate handles the service instance update callback.
func (wm *watcherMgr) OnInstancesUpdate(resp *model.InstancesResponse) {
	services, err := parseInstances(resp.GetInstances())
	if err != nil {
		log.Warnf("%s instances parse failed: %v", wm.serviceName, err)
		return
	}

	wm.rw.Lock()
	wm.serviceInstances = services
	wm.rw.Unlock()

	wm.broadcast()
}

// fork creates a new service instance watcher.
func (wm *watcherMgr) fork() (registry.Watcher, error) {
	wm.rw.Lock()
	defer wm.rw.Unlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	w := newWatcher(wm, wm.idx.Add(1))
	wm.watchers[w.idx] = w

	return w, nil
}

// recycle reclaims a service instance watcher.
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

	wm.removeFromRegistry()
	wm.rw.Unlock()

	wm.unsubscribe()
}

// removeFromRegistry removes this manager from the registry.
//
// It uses CompareAndDelete to atomically verify and remove the manager so that a concurrently
// rebuilt manager is not mistakenly deleted by the cleanup logic of an old manager.
func (wm *watcherMgr) removeFromRegistry() {
	wm.registry.watchers.CompareAndDelete(wm.serviceName, wm)
}

// stop stops watching service instance updates.
func (wm *watcherMgr) stop() {
	wm.rw.Lock()
	if !wm.stopped.CompareAndSwap(false, true) {
		wm.rw.Unlock()
		return
	}

	wm.removeFromRegistry()
	watchers := wm.loadWatchers()
	wm.rw.Unlock()

	for _, w := range watchers {
		w.Stop()
	}

	wm.unsubscribe()
}

// unsubscribe cancels the subscription to service instance changes.
func (wm *watcherMgr) unsubscribe() {
	if wm.resp != nil {
		wm.resp.CancelWatch()
	}
}

// broadcast broadcasts service instance updates.
func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// loadWatchers loads all service instance watchers.
func (wm *watcherMgr) loadWatchers() []*watcher {
	watchers := make([]*watcher, 0, len(wm.watchers))

	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	return watchers
}

// loadServices loads all service instances.
func (wm *watcherMgr) loadServices() []*registry.ServiceInstance {
	services := make([]*registry.ServiceInstance, 0, len(wm.serviceInstances))

	for _, ins := range wm.serviceInstances {
		service := &registry.ServiceInstance{
			ID:       ins.ID,
			Name:     ins.Name,
			Kind:     ins.Kind,
			Alias:    ins.Alias,
			State:    ins.State,
			Events:   make([]int, len(ins.Events)),
			Routes:   make([]registry.Route, len(ins.Routes)),
			Services: make([]string, len(ins.Services)),
			Endpoint: ins.Endpoint,
			Weight:   ins.Weight,
			Metadata: make(map[string]string, len(ins.Metadata)),
		}

		copy(service.Events, ins.Events)
		copy(service.Routes, ins.Routes)
		copy(service.Services, ins.Services)
		maps.Copy(service.Metadata, ins.Metadata)

		services = append(services, service)
	}

	return services
}

// services returns the list of service instances.
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
