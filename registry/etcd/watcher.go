package etcd

import (
	"context"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	stateInitial int32 = iota // 0
	stateRunning              // 1
	stateStopped              // 2
)

const (
	// minRetryDelay is the minimum retry delay.
	minRetryDelay = 100 * time.Millisecond

	// maxRetryDelay is the maximum retry delay.
	maxRetryDelay = 10 * time.Second

	// stableDuration is the link stability threshold: a watch or keepalive stream is considered
	// healthy only after it has lived longer than this duration, at which point the retry delay is
	// reset. This keeps the backoff from being reset repeatedly when streams frequently die early.
	stableDuration = 30 * time.Second

	// resyncInterval is the periodic full reconciliation interval. When a watch stream receives no
	// response for a long time, a full pull is issued to reconcile and probe the link health, so
	// that stale data is not served for long after a silent disconnect.
	resyncInterval = 5 * time.Minute
)

// watcher watches service instances.
//
// It pushes the latest service instance list to the caller through a channel of capacity 1, so
// that only the newest data is kept.
type watcher struct {
	idx     int64                            // Watcher index
	wm      *watcherMgr                      // Owning watch manager
	state   atomic.Int32                     // Watcher state
	mu      sync.Mutex                       // Guards chWatch
	chWatch chan []*registry.ServiceInstance // Notification channel for the service instance list
}

// newWatcher returns a new service instance watcher owned by wm with the given index.
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 1)

	return w
}

// notify tells the watcher that the service instance list has been updated.
//
// It keeps only the latest data: pending unconsumed data in the channel is discarded before the
// new data is written. The channel has capacity 1 and is drained before writing, so the write
// never blocks and cannot race with Stop closing the channel.
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

// flush drains every stale item from the notification channel.
func (w *watcher) flush() {
	for {
		select {
		case <-w.chWatch:
			// continue
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
//
// The first call returns the current snapshot of the service instances and falls back to the
// manager cache when the channel is empty. Later calls block until the service instances change or
// the watch is stopped.
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
//
// It closes the internal notification channel and recycles the owning manager. It is idempotent;
// repeated calls return an illegal operation error.
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
//
// It maintains the watch stream, the local service instance cache and the lifecycle of the derived
// watchers for a service name.
type watcherMgr struct {
	registry         *Registry                            // Service registry
	ctx              context.Context                      // Manager context
	cancel           context.CancelFunc                   // Manager cancel function
	serviceName      string                               // Service name
	watcher          clientv3.Watcher                     // etcd watch client
	watchKey         string                               // Service watch prefix key
	watchChan        clientv3.WatchChan                   // Watch event channel
	idx              atomic.Int64                         // Watcher index counter
	rw               sync.RWMutex                         // Guards watchers and serviceInstances
	watchers         map[int64]*watcher                   // Watcher registry
	wg               sync.WaitGroup                       // Waits for the watch event goroutine to exit
	stopped          atomic.Bool                          // Whether the manager has stopped
	health           atomic.Bool                          // Whether the watch link is healthy
	serviceInstances map[string]*registry.ServiceInstance // Service instance cache
}

// newWatcherMgr returns a new watch manager for serviceName.
//
// It initializes the local cache from the full query result and starts the watch stream from
// revision+1 of that result so that no event is lost, then starts a background goroutine that
// maintains event reception, periodic reconciliation and reconnection of the watch stream.
func newWatcherMgr(r *Registry, serviceName string, res *clientv3.GetResponse) *watcherMgr {
	wm := &watcherMgr{}
	wm.registry = r
	wm.ctx, wm.cancel = context.WithCancel(r.ctx)
	wm.serviceName = serviceName
	wm.watcher = clientv3.NewWatcher(r.opts.client)
	wm.watchers = make(map[int64]*watcher)
	wm.watchKey = buildPrefixKey(r.opts.namespace, serviceName)
	wm.serviceInstances = make(map[string]*registry.ServiceInstance)

	for _, kv := range res.Kvs {
		if service, err := unmarshal(kv.Value); err != nil {
			log.Warnf("etcd watch get failed: %v", err)
		} else {
			wm.serviceInstances[service.ID] = service
		}
	}

	wm.health.Store(true)
	wm.watchChan = wm.watcher.Watch(
		wm.ctx,
		wm.watchKey,
		clientv3.WithPrefix(),
		clientv3.WithRev(res.Header.Revision+1),
	)

	return wm
}

// init initializes the watch stream event goroutine.
func (wm *watcherMgr) init() {
	wm.wg.Go(func() {
		var (
			ok bool

			// Reconnection backoff (minRetryDelay to maxRetryDelay), maintained by this goroutine:
			// - A watch stream that lived longer than stableDuration before breaking is treated as
			//   a transient hiccup, so the backoff is reset for a quick recovery.
			// - A watch stream that broke before becoming stable (no response received or died
			//   soon after being established) or a failed full sync is treated as a persistent
			//   link failure, so exponential backoff avoids busy spinning.
			delay = minRetryDelay
		)

		for {
			if wm.watchLoop() {
				// This watch stream once ran stably, which means the link is healthy; reset the
				// backoff.
				delay = minRetryDelay
			} else if !wm.stopped.Load() {
				// The watch stream broke before becoming stable (no response received or died
				// soon after being established), so count it as a failed round.
				delay = min(delay*2, maxRetryDelay)
			}

			if wm.stopped.Load() {
				return
			}

			// The watch link broke abnormally; mark it unhealthy and keep reconnecting until it
			// succeeds or the watcherMgr stops.
			wm.health.Store(false)

			// Reconnect the watch stream.
			if ok, delay = wm.reconnect(delay); !ok {
				return
			}

			// Reconnection succeeded; restore the healthy state.
			wm.health.Store(true)
		}
	})
}

// fork creates a new watcher.
//
// It derives a watcher from the manager and registers it; it returns an error when the manager has
// stopped.
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

	wm.cancel()
	wm.watcher.Close()
	wm.wg.Wait()
}

// removeFromRegistry removes this manager from the registry.
//
// It only removes the entry when the registry still points to this manager, so that the cleanup of
// an old manager does not delete a concurrently rebuilt one.
func (wm *watcherMgr) removeFromRegistry() {
	reg := wm.registry

	reg.mu1.Lock()
	defer reg.mu1.Unlock()

	if v, ok := reg.watchers.Load(wm.serviceName); ok && v == wm {
		reg.watchers.Delete(wm.serviceName)
	}
}

// stop stops watching.
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
	wm.watcher.Close()
	wm.wg.Wait()
}

// watchLoop is the watch event loop.
//
// Besides receiving watch events, it performs a full pull reconciliation once per resyncInterval:
//   - Reconciliation succeeds: the local cache is refreshed and broadcast, the link health is
//     confirmed and watching continues.
//   - Reconciliation fails: the watch stream may have died silently (for example a half-open
//     connection was not detected in time), so the loop returns and lets the outer layer rebuild it.
//
// It reports whether this stream once ran stably (a response was received and it lived for at least
// stableDuration); the outer loop uses it to decide whether to reset the reconnection backoff.
func (wm *watcherMgr) watchLoop() bool {
	var (
		received bool
		start    = time.Now()
		ticker   = time.NewTicker(resyncInterval)
	)

	defer ticker.Stop()

	// Report whether this stream once ran stably.
	stable := func() bool { return received && time.Since(start) >= stableDuration }

	for {
		select {
		case <-wm.ctx.Done():
			return stable()
		case <-ticker.C:
			if _, err := wm.sync(); err != nil {
				if wm.ctx.Err() != nil {
					return stable()
				}
				log.Warnf("etcd watch resync failed, will rebuild watch stream, err: %v", err)
				return stable()
			}
		case res, ok := <-wm.watchChan:
			if !ok {
				return stable()
			}

			if res.Err() != nil {
				log.Warnf("etcd watch error: %v", res.Err())
				return stable()
			}

			received = true

			// Deserialize events outside the lock first to shorten the write-lock hold time and
			// avoid blocking concurrent reads.
			updates := make([]*registry.ServiceInstance, 0, len(res.Events))
			deletes := make([]string, 0, len(res.Events))
			for _, ev := range res.Events {
				switch ev.Type {
				case mvccpb.PUT:
					if service, err := unmarshal(ev.Kv.Value); err == nil {
						updates = append(updates, service)
					} else {
						log.Warnf("etcd watch put failed: %v", err)
					}
				case mvccpb.DELETE:
					if id, ok := strings.CutPrefix(string(ev.Kv.Key), wm.watchKey); ok {
						deletes = append(deletes, id)
					} else {
						log.Warnf("etcd watch delete key %s failed", ev.Kv.Key)
					}
				}
			}

			wm.rw.Lock()
			for _, service := range updates {
				wm.serviceInstances[service.ID] = service
			}
			for _, id := range deletes {
				delete(wm.serviceInstances, id)
			}
			wm.rw.Unlock()

			wm.broadcast()
		}
	}
}

// sync pulls the full service data and refreshes the local cache, then broadcasts the latest data
// on success.
func (wm *watcherMgr) sync() (*clientv3.GetResponse, error) {
	tctx, tcancel := context.WithTimeout(wm.ctx, wm.registry.opts.timeout)
	res, err := wm.registry.opts.client.Get(tctx, wm.watchKey, clientv3.WithPrefix())
	tcancel()
	if err != nil {
		return nil, err
	}

	wm.rw.Lock()
	wm.serviceInstances = make(map[string]*registry.ServiceInstance)
	for _, kv := range res.Kvs {
		if service, err := unmarshal(kv.Value); err == nil {
			wm.serviceInstances[service.ID] = service
		} else {
			log.Warnf("etcd watch resync put failed: %v", err)
		}
	}
	wm.rw.Unlock()

	// Replay the full service data after a successful sync.
	wm.broadcast()

	return res, nil
}

// reconnect pulls the full service data and rebuilds the watch stream after a disconnect.
//
// It retries with exponential backoff capped at maxRetryDelay and does not destroy the watcher on a
// transient failure. After a successful reconnection it replays the full service data and rebuilds
// the watch from revision+1 of the Get response so that no event is lost. It returns false only
// when the watcherMgr has stopped or the context is done.
func (wm *watcherMgr) reconnect(delay time.Duration) (bool, time.Duration) {
	for {
		if wm.stopped.Load() {
			return false, delay
		}

		select {
		case <-wm.ctx.Done():
			return false, delay
		case <-time.After(delay):
		}

		res, err := wm.sync()
		if err != nil {
			if wm.ctx.Err() != nil {
				return false, delay
			}
			delay = min(delay*2, maxRetryDelay)
			log.Warnf("etcd watch reconnect failed, will retry after %v, err: %v", delay, err)
			continue
		}

		// Rebuild the watch from revision+1 of the Get response so that no event is lost.
		wm.watchChan = wm.watcher.Watch(
			wm.ctx,
			wm.watchKey,
			clientv3.WithPrefix(),
			clientv3.WithRev(res.Header.Revision+1),
		)

		return true, delay
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

// loadWatchers loads every watcher.
func (wm *watcherMgr) loadWatchers() []*watcher {
	watchers := make([]*watcher, 0, len(wm.watchers))

	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	return watchers
}

// loadServices loads every service instance.
//
// It deep-copies the cached instances before returning them, so that caller modifications do not
// pollute the cache.
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
//
// It returns an error when the manager has stopped.
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
