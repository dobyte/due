/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/16 10:26 下午
 * @Desc: TODO
 */

package etcd

import (
	"strings"
	"sync"
	"sync/atomic"

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

// 通知监听器服务实例列表已更新
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

// 清空所有旧数据
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

// Next 返回服务实例列表
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

type watcherMgr struct {
	registry         *Registry
	serviceName      string
	watcher          clientv3.Watcher
	chWatch          clientv3.WatchChan
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
	wg               sync.WaitGroup
	stopped          atomic.Bool
	serviceInstances map[string]*registry.ServiceInstance
}

func newWatcherMgr(r *Registry, serviceName string, res *clientv3.GetResponse) *watcherMgr {
	wm := &watcherMgr{}
	wm.registry = r
	wm.serviceName = serviceName
	wm.watcher = clientv3.NewWatcher(r.opts.client)
	wm.watchers = make(map[int64]*watcher)
	wm.serviceInstances = make(map[string]*registry.ServiceInstance)

	for _, kv := range res.Kvs {
		if service, err := unmarshal(kv.Value); err != nil {
			log.Warnf("etcd watch get failed: %v", err)
		} else {
			wm.serviceInstances[service.ID] = service
		}
	}

	wm.chWatch = wm.watcher.Watch(
		r.ctx,
		buildPrefixKey(wm.registry.opts.namespace, wm.serviceName),
		clientv3.WithPrefix(),
		clientv3.WithRev(res.Header.Revision+1),
	)

	wm.wg.Go(func() {
		for {
			res, ok := <-wm.chWatch
			if !ok {
				return
			}

			if res.Err() != nil {
				return
			}

			wm.rw.Lock()
			for _, ev := range res.Events {
				switch ev.Type {
				case mvccpb.PUT:
					if service, err := unmarshal(ev.Kv.Value); err == nil {
						wm.serviceInstances[service.ID] = service
					} else {
						log.Warnf("etcd watch put failed: %v", err)
					}
				case mvccpb.DELETE:
					if parts := strings.Split(string(ev.Kv.Key), "/"); len(parts) == 4 {
						delete(wm.serviceInstances, parts[3])
					} else {
						log.Warnf("etcd watch delete key %s failed", ev.Kv.Key)
					}
				}
			}
			wm.rw.Unlock()

			wm.broadcast()
		}
	})

	return wm
}

// 创建新监听器
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

// 回收监听器
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

	wm.watcher.Close()
	wm.wg.Wait()
}

// 停止监听
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

	wm.watcher.Close()
	wm.wg.Wait()
}

// 广播服务实例列表
func (wm *watcherMgr) broadcast() {
	wm.rw.RLock()
	services := wm.loadServices()
	watchers := wm.loadWatchers()
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// 返回所有监听器
func (wm *watcherMgr) loadWatchers() []*watcher {
	watchers := make([]*watcher, 0, len(wm.watchers))

	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	return watchers
}

// 返回所有服务实例
func (wm *watcherMgr) loadServices() []*registry.ServiceInstance {
	services := make([]*registry.ServiceInstance, 0)

	for k := range wm.serviceInstances {
		service := wm.serviceInstances[k]
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
