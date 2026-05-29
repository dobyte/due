package consul

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

type state int32

const (
	stateInitial state = 0
	stateRunning state = 1
	stateStopped state = 2
)

type watcher struct {
	idx        int64
	ctx        context.Context
	cancel     context.CancelFunc
	watcherMgr *watcherMgr
	rw         sync.RWMutex
	state      state
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
	w.rw.RLock()
	defer w.rw.RUnlock()

	if w.state == stateRunning {
		w.chWatch <- services
	}
}

// Next 返回服务实例列表
func (w *watcher) Next() <-chan []*registry.ServiceInstance {
	w.rw.Lock()
	if w.state == stateInitial {
		w.state = stateRunning
		w.chWatch <- w.watcherMgr.services()
	}
	w.rw.Unlock()

	return w.chWatch
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.rw.Lock()
	defer w.rw.Unlock()

	if w.state == stateStopped {
		return errors.ErrIllegalOperation
	}

	w.state = stateStopped
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
