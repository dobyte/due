/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/16 10:26 下午
 * @Desc: TODO
 */

package etcd

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
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
	idx        int64
	ctx        context.Context
	cancel     context.CancelFunc
	watcherMgr *watcherMgr
	state      atomic.Int32 // 状态，无锁读写
	mu         sync.Mutex   // 协调 chWatch 的 send 与 close
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
		return w.watcherMgr.services(), nil
	}

	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case services, ok := <-w.chWatch:
		if !ok {
			return nil, errors.ErrWatcherStopped
		}

		return services, nil
	}
}

// Stop 停止监听
func (w *watcher) Stop() error {
	if w.state.Swap(stateStopped) == stateStopped {
		return errors.ErrIllegalOperation
	}

	w.cancel()

	w.mu.Lock()
	close(w.chWatch)
	w.mu.Unlock()

	w.watcherMgr.recycle(w.idx)

	return nil
}

type watcherMgr struct {
	ctx              context.Context
	cancel           context.CancelFunc
	registry         *Registry
	serviceName      string
	serviceInstances sync.Map
	watcher          clientv3.Watcher
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
	wg               sync.WaitGroup
	stopped          atomic.Bool
}

func newWatcherMgr(registry *Registry, serviceName string, services []*registry.ServiceInstance) *watcherMgr {
	w := &watcherMgr{}
	w.ctx, w.cancel = context.WithCancel(registry.ctx)
	w.registry = registry
	w.serviceName = serviceName
	w.watcher = clientv3.NewWatcher(registry.opts.client)
	w.watchers = make(map[int64]*watcher)

	for _, service := range services {
		w.serviceInstances.Store(service.ID, service)
	}

	w.wg.Go(func() {
		w.watchLoop()
	})

	return w
}

// 监听循环，出错自动重试
func (wm *watcherMgr) watchLoop() {
	prefix := buildPrefixKey(wm.registry.opts.namespace, wm.serviceName)
	backoff := 100 * time.Millisecond
	maxBackoff := 10 * time.Second

	for {
		select {
		case <-wm.ctx.Done():
			return
		default:
		}

		ch := wm.watcher.Watch(wm.ctx, prefix, clientv3.WithPrefix())

		err := wm.watchEvents(ch)
		if err == nil {
			return
		}

		if wm.ctx.Err() != nil {
			return
		}

		services, sErr := wm.registry.services(wm.ctx, wm.serviceName)
		if sErr == nil {
			wm.serviceInstances.Range(func(key, value any) bool {
				wm.serviceInstances.Delete(key)
				return true
			})
			for _, service := range services {
				wm.serviceInstances.Store(service.ID, service)
			}
			wm.broadcast()
		}

		select {
		case <-time.After(backoff):
		case <-wm.ctx.Done():
			return
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// 处理单次 watch 的事件，返回错误表示需要重连
func (wm *watcherMgr) watchEvents(ch clientv3.WatchChan) error {
	for {
		select {
		case <-wm.ctx.Done():
			return nil
		case res, ok := <-ch:
			if !ok {
				return errors.ErrClientClosed
			}

			if res.Err() != nil {
				return res.Err()
			}

			for _, ev := range res.Events {
				switch ev.Type {
				case mvccpb.PUT:
					if service, err := unmarshal(ev.Kv.Value); err == nil {
						wm.serviceInstances.Store(service.ID, service)
					}
				case mvccpb.DELETE:
					if parts := strings.Split(string(ev.Kv.Key), "/"); len(parts) == 4 {
						wm.serviceInstances.Delete(parts[3])
					}
				}
			}

			wm.broadcast()
		}
	}
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

	wm.cancel()
	wm.wg.Wait()
	wm.watcher.Close()
}

// 停止监听
func (wm *watcherMgr) stop() {
	if !wm.stopped.CompareAndSwap(false, true) {
		return
	}

	wm.cancel()
	wm.wg.Wait()
	wm.watcher.Close()
}

// 广播服务实例列表
func (wm *watcherMgr) broadcast() {
	services := wm.services()

	wm.rw.RLock()
	watchers := make([]*watcher, 0, len(wm.watchers))
	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(services)
	}
}

// 返回所有服务实例
func (wm *watcherMgr) services() []*registry.ServiceInstance {
	services := make([]*registry.ServiceInstance, 0)

	wm.serviceInstances.Range(func(key, value any) bool {
		services = append(services, value.(*registry.ServiceInstance))
		return true
	})

	return services
}
