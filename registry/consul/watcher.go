package consul

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xcall"
)

// Watcher states.
const (
	stateInitial int32 = iota // Initial state
	stateRunning              // Running
	stateStopped              // Stopped
)

// watcher is a service instance watcher that obtains service instance updates through
// [watcher.Next].
type watcher struct {
	idx     int64                            // Watcher sequence number
	wm      *watcherMgr                      // Owning watcher manager
	state   atomic.Int32                     // Watcher state
	mu      sync.Mutex                       // Protects the closing and sending of chWatch
	chWatch chan []*registry.ServiceInstance // Service instance update channel
}

// newWatcher creates a service instance watcher.
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 1)

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

// flush empties the watch queue.
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

// Stop stops watching.
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

// watcherMgr is a service instance watcher manager that manages all watchers under the same service
// name and keeps a snapshot of the service instances.
type watcherMgr struct {
	registry         *Registry                   // Owning registry and discovery component
	ctx              context.Context             // Watch goroutine context
	cancel           context.CancelFunc          // Watch goroutine cancel function
	serviceName      string                      // Service name
	idx              atomic.Int64                // Watcher sequence number generator
	rw               sync.RWMutex                // Protects the watchers, serviceInstances and other fields
	watchers         map[int64]*watcher          // Watcher registry
	wg               sync.WaitGroup              // Waits for the watch goroutine to exit
	stopped          atomic.Bool                 // Whether the manager has stopped
	healthy          atomic.Bool                 // Whether the watch connection is healthy
	serviceInstances []*registry.ServiceInstance // Service instance snapshot
	serviceWaitIndex uint64                      // Service instance query index
}

// newWatcherMgr creates a service instance watcher manager.
func newWatcherMgr(r *Registry, serviceName string, services []*registry.ServiceInstance, waitIndex uint64) *watcherMgr {
	wm := &watcherMgr{}
	wm.registry = r
	wm.ctx, wm.cancel = context.WithCancel(context.Background())
	wm.serviceName = serviceName
	wm.watchers = make(map[int64]*watcher)
	wm.serviceInstances = services
	wm.serviceWaitIndex = waitIndex
	wm.healthy.Store(true)

	return wm
}

// init initializes the service instance watcher.
func (wm *watcherMgr) init() {
	wm.wg.Go(func() {
		for {
			wm.watchLoop()

			if wm.stopped.Load() {
				return
			}

			if !wm.resyncWithRetry() {
				wm.rw.Lock()
				watchers := wm.loadWatchers()

				if wm.stopped.CompareAndSwap(false, true) {
					wm.removeFromRegistry()
					wm.rw.Unlock()

					for _, w := range watchers {
						w.Stop()
					}

					wm.cancel()

					return
				}

				wm.rw.Unlock()

				return
			}
		}
	})
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

	wm.cancel()
	wm.wg.Wait()
}

// removeFromRegistry removes this manager from the registry.
//
// It removes the manager only when the registry still points to it, so that a concurrently rebuilt
// manager is not mistakenly deleted by the cleanup logic of an old manager.
func (wm *watcherMgr) removeFromRegistry() {
	reg := wm.registry

	reg.mu1.Lock()
	defer reg.mu1.Unlock()

	if v, ok := reg.watchers.Load(wm.serviceName); ok && v == wm {
		reg.watchers.Delete(wm.serviceName)
	}
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

	wm.cancel()
	wm.wg.Wait()
}

// watchLoop watches for service instance updates.
func (wm *watcherMgr) watchLoop() {
	for {
		select {
		case <-wm.ctx.Done():
			return
		default:
		}

		services, index, err := wm.registry.services(wm.ctx, wm.serviceName, wm.serviceWaitIndex, true, true)

		if err != nil {
			if wm.ctx.Err() != nil {
				return
			}
			wm.healthy.Store(false)
			log.Warnf("consul watch error: %v", err)
			return
		}

		wm.healthy.Store(true)

		wm.rw.Lock()
		if index != wm.serviceWaitIndex {
			wm.serviceWaitIndex = index
			wm.serviceInstances = services
			wm.rw.Unlock()

			wm.broadcast()
		} else {
			wm.rw.Unlock()
		}
	}
}

// resyncWithRetry retries synchronizing the service instances.
func (wm *watcherMgr) resyncWithRetry() bool {
	err := xcall.Backoff(wm.ctx, func(ctx context.Context, attempt int) (bool, error) {
		if wm.stopped.Load() {
			return false, errors.ErrWatcherStopped
		}

		ctx, cancel := context.WithTimeout(ctx, wm.registry.opts.timeout)
		services, index, err := wm.registry.services(ctx, wm.serviceName, 0, true, false)
		cancel()
		if err != nil {
			log.Warnf("consul watch resync failed, retry %d times, err: %v", attempt, err)
			return true, err
		}

		wm.rw.Lock()
		wm.serviceInstances = services
		wm.serviceWaitIndex = index
		wm.rw.Unlock()

		wm.healthy.Store(true)
		wm.broadcast()

		return false, nil
	}, max(1, wm.registry.opts.retryTimes), 100*time.Millisecond, 3*time.Second)

	return err == nil
}

// broadcast notifies the watchers of service instance updates.
func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// loadWatchers loads all watchers.
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

// services returns all service instances.
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
