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

func (w *watcher) notify(services []*registry.ServiceInstance) {
	if w.state.Load() != stateRunning {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state.Load() != stateRunning {
		return
	}

	select {
	case w.chWatch <- services:
	default:
		// channel 满，丢弃一条旧数据后重试
		select {
		case <-w.chWatch:
		default:
		}
		select {
		case w.chWatch <- services:
		default:
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

	return w.watcherMgr.recycle(w.idx)
}

type watcherMgr struct {
	ctx              context.Context
	cancel           context.CancelFunc
	registry         *Registry
	serviceName      string
	serviceInstances sync.Map
	watcher          clientv3.Watcher
	chWatch          clientv3.WatchChan
	idx              atomic.Int64
	rw               sync.RWMutex
	watchers         map[int64]*watcher
	wg               sync.WaitGroup
}

func newWatcherMgr(r *Registry, ctx context.Context, serviceName string) (*watcherMgr, error) {
	services, err := r.services(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	w := &watcherMgr{}
	w.ctx, w.cancel = context.WithCancel(r.ctx)
	w.registry = r
	w.serviceName = serviceName
	w.watcher = clientv3.NewWatcher(r.opts.client)
	w.chWatch = w.watcher.Watch(w.ctx, buildPrefixKey(r.opts.namespace, w.serviceName), clientv3.WithPrefix())
	w.watchers = make(map[int64]*watcher)

	for _, service := range services {
		w.serviceInstances.Store(service.ID, service)
	}

	w.wg.Go(func() {
		for {
			select {
			case <-w.ctx.Done():
				return
			case res, ok := <-w.chWatch:
				if !ok {
					return
				}

				if res.Err() != nil {
					return
				}

				for _, ev := range res.Events {
					switch ev.Type {
					case mvccpb.PUT:
						if service, err := unmarshal(ev.Kv.Value); err == nil {
							w.serviceInstances.Store(service.ID, service)
						}
					case mvccpb.DELETE:
						if parts := strings.Split(string(ev.Kv.Key), "/"); len(parts) == 4 {
							w.serviceInstances.Delete(parts[3])
						}
					}
				}

				w.broadcast()
			}
		}
	})

	return w, nil
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
		wm.wg.Wait()
		wm.registry.watchers.Delete(wm.serviceName)
		return wm.watcher.Close()
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

func (wm *watcherMgr) services() (services []*registry.ServiceInstance) {
	wm.serviceInstances.Range(func(key, value any) bool {
		services = append(services, value.(*registry.ServiceInstance))
		return true
	})
	return
}
