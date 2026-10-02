package nacos

import (
	"maps"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// Watcher running state constants.
const (
	stateInitial int32 = iota
	stateRunning
	stateStopped
)

// watcher watches service instances.
type watcher struct {
	idx     int64
	wm      *watcherMgr
	state   atomic.Int32
	mu      sync.Mutex
	chWatch chan []*registry.ServiceInstance
}

// newWatcher returns a new service instance watcher owned by wm with the given index.
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 16)

	return w
}

// notify tells the watcher that the service instances have been updated.
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

// flush drains every item from the notification channel.
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

// Next returns the service instance list.
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

// watcherMgr manages the watchers of one service.
type watcherMgr struct {
	registry         *Registry
	serviceName      string
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
	stopped          atomic.Bool
	serviceInstances []*registry.ServiceInstance
	param            *vo.SubscribeParam
}

// newWatcherMgr returns a new watch manager for serviceName.
func newWatcherMgr(registry *Registry, serviceName string, services []*registry.ServiceInstance) *watcherMgr {
	wm := &watcherMgr{}
	wm.registry = registry
	wm.serviceName = serviceName
	wm.watchers = make(map[int64]*watcher)
	wm.serviceInstances = services

	return wm
}

// init initializes service instance watching.
func (wm *watcherMgr) init() error {
	wm.param = &vo.SubscribeParam{
		ServiceName:       wm.serviceName,
		Clusters:          []string{wm.registry.opts.clusterName},
		GroupName:         wm.registry.opts.groupName,
		SubscribeCallback: wm.callback,
	}

	if err := wm.subscribe(); err != nil {
		wm.unsubscribe()
		return err
	}

	return nil
}

func (wm *watcherMgr) subscribe() error {
	if err := wm.registry.opts.client.Subscribe(wm.param); err != nil {
		return err
	}

	return nil
}

// callback handles the service instance update callback.
func (wm *watcherMgr) callback(instances []model.Instance, err error) {
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

// recycle recycles the watcher with the given index.
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
// It uses CompareAndDelete to validate and remove atomically, so that a concurrently rebuilt
// manager is not deleted by the cleanup of an old one.
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

// unsubscribe unsubscribes from service instance updates.
func (wm *watcherMgr) unsubscribe() {
	if err := wm.registry.opts.client.Unsubscribe(wm.param); err != nil {
		log.Warnf("%s unsubscribe failed: %v", wm.serviceName, err)
	}
}

// broadcast notifies the watchers of the service instance update.
func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// loadWatchers loads every service instance watcher.
func (wm *watcherMgr) loadWatchers() []*watcher {
	watchers := make([]*watcher, 0, len(wm.watchers))

	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	return watchers
}

// loadServices loads every service instance.
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

// services returns every service instance.
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
