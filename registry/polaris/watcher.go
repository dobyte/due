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

// 监听器运行状态常量
const (
	stateInitial int32 = iota
	stateRunning
	stateStopped
)

// watcher 服务实例监听器
type watcher struct {
	idx     int64
	wm      *watcherMgr
	state   atomic.Int32
	mu      sync.Mutex
	chWatch chan []*registry.ServiceInstance
}

// newWatcher 创建服务实例监听器
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 16)

	return w
}

// notify 通知服务实例更新
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

// flush 清空通道中的数据
func (w *watcher) flush() {
	for {
		select {
		case <-w.chWatch:
		default:
			return
		}
	}
}

// latest 获取最新的服务实例
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

// watcherMgr 服务实例监听管理器
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

// newWatcherMgr 创建服务实例监听管理器
func newWatcherMgr(registry *Registry, serviceName string) *watcherMgr {
	wm := &watcherMgr{}
	wm.registry = registry
	wm.serviceName = serviceName
	wm.watchers = make(map[int64]*watcher)

	return wm
}

// init 初始化服务实例监听
func (wm *watcherMgr) init() error {
	if err := wm.subscribe(); err != nil {
		wm.unsubscribe()
		return err
	}

	// 订阅成功后以订阅时刻的快照初始化服务实例，避免遗漏订阅前发生的变更
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

// subscribe 订阅服务实例变更
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

// OnInstancesUpdate 处理服务实例更新回调
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

// fork 创建新的服务实例监听器
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

// recycle 回收服务实例监听器
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

// removeFromRegistry 从注册表中移除本管理器
// 使用 CompareAndDelete 原子地校验并移除，避免并发重建的新管理器被旧管理器的清理逻辑误删
func (wm *watcherMgr) removeFromRegistry() {
	wm.registry.watchers.CompareAndDelete(wm.serviceName, wm)
}

// stop 停止监听服务实例更新
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

// unsubscribe 取消订阅服务实例变更
func (wm *watcherMgr) unsubscribe() {
	if wm.resp != nil {
		wm.resp.CancelWatch()
	}
}

// broadcast 广播服务实例更新
func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// loadWatchers 加载所有服务实例监听器
func (wm *watcherMgr) loadWatchers() []*watcher {
	watchers := make([]*watcher, 0, len(wm.watchers))

	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	return watchers
}

// loadServices 加载所有服务实例
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

// services 获取服务实例列表
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
