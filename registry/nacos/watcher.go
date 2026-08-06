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

// 通知服务实例更新
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

// 清空通道中的数据
func (w *watcher) flush() {
	for {
		select {
		case <-w.chWatch:
		default:
			return
		}
	}
}

// 获取最新的服务实例
func (w *watcher) latest() ([]*registry.ServiceInstance, error) {
	var (
		exist     bool
		instances []*registry.ServiceInstance
	)

	for {
		select {
		case services, ok := <-w.chWatch:
			if !ok && !exist {
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

// Next 返回服务实例列表
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

// Stop 停止监听服务实例更新
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

// 处理服务实例更新回调
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

// 订阅服务实例更新
func (wm *watcherMgr) subscribe() error {
	return wm.registry.opts.client.Subscribe(&vo.SubscribeParam{
		ServiceName:       wm.serviceName,
		Clusters:          []string{wm.registry.opts.clusterName},
		GroupName:         wm.registry.opts.groupName,
		SubscribeCallback: wm.callback,
	})
}

// 创建新的服务实例监听器
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

// 回收服务实例监听器
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

// 停止监听服务实例更新
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

// 取消订阅服务实例更新
func (wm *watcherMgr) unsubscribe() {
	if err := wm.registry.opts.client.Unsubscribe(&vo.SubscribeParam{
		ServiceName:       wm.serviceName,
		Clusters:          []string{wm.registry.opts.clusterName},
		GroupName:         wm.registry.opts.groupName,
		SubscribeCallback: wm.callback,
	}); err != nil {
		log.Warnf("%s unsubscribe failed: %v", wm.serviceName, err)
	}
}

// 广播服务实例更新
func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// 加载所有服务实例
func (wm *watcherMgr) loadWatchers() []*watcher {
	watchers := make([]*watcher, 0, len(wm.watchers))

	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	return watchers
}

// 加载所有服务实例
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

// 加载所有服务实例
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
