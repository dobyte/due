package consul

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
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
	ctx              context.Context
	cancel           context.CancelFunc
	serviceName      string
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
	wg               sync.WaitGroup
	stopped          atomic.Bool
	err              error
	serviceInstances []*registry.ServiceInstance
	serviceWaitIndex uint64
}

func newWatcherMgr(r *Registry, serviceName string, services []*registry.ServiceInstance, waitIndex uint64) *watcherMgr {
	wm := &watcherMgr{}
	wm.registry = r
	wm.ctx, wm.cancel = context.WithCancel(context.Background())
	wm.serviceName = serviceName
	wm.watchers = make(map[int64]*watcher)
	wm.serviceInstances = services
	wm.serviceWaitIndex = waitIndex

	wm.wg.Go(func() {
		for {
			wm.watchLoop()

			if wm.stopped.Load() {
				return
			}

			if !wm.resyncWithRetry() {
				wm.rw.Lock()
				wm.err = errors.ErrWatcherStopped
				watchers := wm.loadWatchers()
				wm.rw.Unlock()

				for _, w := range watchers {
					w.Stop()
				}
				return
			}
		}
	})

	return wm
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

	wm.cancel()
	wm.wg.Wait()
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

	wm.cancel()
	wm.wg.Wait()
}

func (wm *watcherMgr) watchLoop() {
	for {
		select {
		case <-wm.ctx.Done():
			return
		default:
		}

		ctx, cancel := context.WithTimeout(wm.ctx, 120*time.Second)
		services, index, err := wm.registry.services(ctx, wm.serviceName, wm.serviceWaitIndex, true)
		cancel()

		if err != nil {
			if wm.ctx.Err() != nil {
				return
			}
			log.Warnf("consul watch error: %v", err)
			return
		}

		if index != wm.serviceWaitIndex {
			wm.rw.Lock()
			wm.serviceWaitIndex = index
			wm.serviceInstances = services
			wm.rw.Unlock()

			wm.broadcast()
		}
	}
}

func (wm *watcherMgr) resyncWithRetry() bool {
	for i := 0; i < wm.registry.opts.retryTimes; i++ {
		select {
		case <-wm.ctx.Done():
			return false
		case <-time.After(wm.registry.opts.retryInterval):
			if wm.stopped.Load() {
				return false
			}

			ctx, cancel := context.WithTimeout(wm.ctx, wm.registry.opts.timeout)
			services, index, err := wm.registry.services(ctx, wm.serviceName, 0, true)
			cancel()
			if err != nil {
				log.Warnf("consul watch resync failed, retry %d times, err: %v", i+1, err)
				continue
			}

			wm.rw.Lock()
			wm.serviceInstances = services
			wm.serviceWaitIndex = index
			wm.rw.Unlock()

			wm.broadcast()

			return true
		}
	}

	return false
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
