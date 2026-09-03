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

// 监听器状态
const (
	stateInitial int32 = iota // 初始状态
	stateRunning              // 运行中
	stateStopped              // 已停止
)

// watcher 服务实例监听器，通过 Next 方法获取服务实例更新。
type watcher struct {
	idx     int64                            // 监听器序号
	wm      *watcherMgr                      // 所属监听管理器
	state   atomic.Int32                     // 监听器状态
	mu      sync.Mutex                       // 保护 chWatch 的关闭与发送
	chWatch chan []*registry.ServiceInstance // 服务实例更新通道
}

// newWatcher 创建服务实例监听器。
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 1)

	return w
}

// 通知监听器服务实例更新
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

// 清空监听队列
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

// Stop 停止监听
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

// watcherMgr 服务实例监听管理器，管理同一服务名下所有监听器，并维护服务实例快照。
type watcherMgr struct {
	registry         *Registry                   // 所属注册发现组件
	ctx              context.Context             // 监听协程上下文
	cancel           context.CancelFunc          // 监听协程取消函数
	serviceName      string                      // 服务名称
	idx              atomic.Int64                // 监听器序号生成器
	rw               sync.RWMutex                // 保护 watchers、serviceInstances 等字段
	watchers         map[int64]*watcher          // 监听器注册表
	wg               sync.WaitGroup              // 等待监听协程退出
	stopped          atomic.Bool                 // 是否已停止
	healthy          atomic.Bool                 // 监听连接是否健康
	serviceInstances []*registry.ServiceInstance // 服务实例快照
	serviceWaitIndex uint64                      // 服务实例查询索引
}

// newWatcherMgr 创建服务实例监听管理器。
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

// 初始化服务实例监听器
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

	wm.removeFromRegistry()
	wm.rw.Unlock()

	wm.cancel()
	wm.wg.Wait()
}

// 从注册表中移除本管理器
// 仅在注册表中仍指向本管理器时才移除，避免并发重建的新管理器被旧管理器的清理逻辑误删
func (wm *watcherMgr) removeFromRegistry() {
	reg := wm.registry

	reg.mu1.Lock()
	defer reg.mu1.Unlock()

	if v, ok := reg.watchers.Load(wm.serviceName); ok && v == wm {
		reg.watchers.Delete(wm.serviceName)
	}
}

// 停止监听服务实例更新
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

// 监听服务实例更新
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

// 重试同步服务实例
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

// 通知监听器服务实例更新
func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// 加载所有监听器
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

// 返回所有服务实例
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
