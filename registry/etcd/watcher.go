/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/16 10:26 下午
 * @Desc: TODO
 */

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
	// minRetryDelay 重试最小等待间隔
	minRetryDelay = 100 * time.Millisecond

	// maxRetryDelay 重试最大等待间隔
	maxRetryDelay = 10 * time.Second

	// stableDuration 链路稳定判定时长：watch 流/保活流存活超过该时长，
	// 才认为链路已恢复健康并重置重试间隔，防止流频繁短命断开时退避被反复重置而失效
	stableDuration = 30 * time.Second

	// resyncInterval 周期全量对账间隔：watch 流长时间无任何响应时，
	// 主动全量拉取对账并探测链路健康，避免静默断链后长期提供过期数据
	resyncInterval = 5 * time.Minute
)

// watcher 服务实例监听器
// 通过容量为1的通道向调用方推送最新的服务实例列表，只保留最新数据
type watcher struct {
	idx     int64                            // 监听器序号
	wm      *watcherMgr                      // 所属的监听管理器
	state   atomic.Int32                     // 监听器状态
	mu      sync.Mutex                       // 保护 chWatch 通道
	chWatch chan []*registry.ServiceInstance // 服务实例列表通知通道
}

// 构建服务实例监听器
// @param wm *watcherMgr 所属的监听管理器
// @param idx int64 监听器序号
// @return @1 *watcher 服务实例监听器实例
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.wm = wm
	w.idx = idx
	w.chWatch = make(chan []*registry.ServiceInstance, 1)

	return w
}

// 通知监听器服务实例列表已更新
// 采用"仅保留最新"语义：先丢弃通道中尚未被消费的旧数据再写入新数据；
// 通道容量为1且写入前已排空，故写入永不阻塞，避免与 Stop 关闭通道形成竞态
// @param services []*registry.ServiceInstance 最新的服务实例列表
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
// 首次调用返回当前最新的服务实例快照（通道为空时回退管理器缓存）；
// 后续调用阻塞等待，直至服务实例发生变更或监听被停止
// @return @1 []*registry.ServiceInstance 服务实例列表
// @return @2 error 监听停止时返回的错误
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
// 关闭内部通知通道并回收所属管理器；幂等，重复调用返回非法操作错误
// @return @1 error 重复停止时返回的错误
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

// watcherMgr 服务监听管理器
// 负责维护同一服务名下的 watch 流、本地服务实例缓存及派生监听器的生命周期
type watcherMgr struct {
	registry         *Registry                            // 服务注册中心
	ctx              context.Context                      // 管理器上下文
	cancel           context.CancelFunc                   // 管理器取消函数
	serviceName      string                               // 服务名称
	watcher          clientv3.Watcher                     // etcd watch 客户端
	watchKey         string                               // 服务监听前缀键
	watchChan        clientv3.WatchChan                   // watch 事件通道
	idx              atomic.Int64                         // 监听器序号计数器
	rw               sync.RWMutex                         // 保护 watchers/serviceInstances
	watchers         map[int64]*watcher                   // 监听器注册表
	wg               sync.WaitGroup                       // 等待 watch 事件协程退出
	stopped          atomic.Bool                          // 是否已停止
	health           atomic.Bool                          // watch 链路是否健康
	serviceInstances map[string]*registry.ServiceInstance // 服务实例缓存
}

// 构建服务监听管理器
// 从全量查询结果初始化本地缓存，并从其 revision+1 处建立 watch 流避免事件丢失；
// 随后启动后台协程统一维护 watch 流的接收、周期对账与断线重建
// @param r *Registry 服务注册中心
// @param serviceName string 服务名称
// @param res *clientv3.GetResponse 全量查询结果
// @return @1 *watcherMgr 服务监听管理器实例
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

// 初始化 初始化 watch 流事件协程
func (wm *watcherMgr) init() {
	wm.wg.Go(func() {
		var (
			ok bool

			// 重连退避间隔（minRetryDelay ~ maxRetryDelay），由本协程统一维护：
			// - watch 流稳定存活超过 stableDuration 后断开，视为瞬时抖动，重置退避快速恢复；
			// - watch 流未稳定即断开（未收到响应或短命断开）或全量同步失败，
			//   视为链路持续异常，指数退避防空转
			delay = minRetryDelay
		)

		for {
			if wm.watchLoop() {
				// 本次 watch 流曾稳定运行，说明链路健康，重置退避间隔
				delay = minRetryDelay
			} else if !wm.stopped.Load() {
				// watch 流未稳定即断开（未收到响应或建立后短命断开），视为一次失败轮次
				delay = min(delay*2, maxRetryDelay)
			}

			if wm.stopped.Load() {
				return
			}

			// watch 链路异常断开，标记为不健康并持续重连直至成功或 watcherMgr 停止
			wm.health.Store(false)

			// 重连 watch 流
			if ok, delay = wm.reconnect(delay); !ok {
				return
			}

			// 重连成功，恢复健康状态
			wm.health.Store(true)
		}
	})
}

// 创建新监听器
// 从管理器派生一个监听器并注册；管理器已停止时返回错误
// @return @1 registry.Watcher 服务实例监听器
// @return @2 error 管理器已停止时返回的错误
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

	wm.removeFromRegistry()
	wm.rw.Unlock()

	wm.cancel()
	wm.watcher.Close()
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

// 停止监听
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

// watch 事件循环
// 除接收 watch 事件外，还按 resyncInterval 周期执行一次全量拉取对账：
// - 对账成功：刷新本地缓存并广播，同时确认链路健康，继续监听；
// - 对账失败：说明 watch 流可能已静默死亡（如半开连接未被及时感知），主动退出交由外层重建
// @return 本次流是否"曾稳定运行"（收到过响应且存活时长 >= stableDuration）；
// 用于外层循环决定是否重置重连退避间隔
func (wm *watcherMgr) watchLoop() bool {
	var (
		received bool
		start    = time.Now()
		ticker   = time.NewTicker(resyncInterval)
	)

	defer ticker.Stop()

	// 判定本次流是否曾稳定运行
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

			// 先在锁外完成事件反序列化，缩短写锁持有时间，避免阻塞并发读操作
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
					if parts := strings.Split(string(ev.Kv.Key), "/"); len(parts) == 4 {
						deletes = append(deletes, parts[3])
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

// 全量拉取服务数据并刷新本地缓存，成功后广播最新数据
// @return @1 *clientv3.GetResponse etcd 全量拉取结果，失败时为 nil
// @return @2 error 拉取失败原因（含上下文取消）
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

	// 同步成功后重播全量服务数据
	wm.broadcast()

	return res, nil
}

// 断线重连：全量拉取服务数据并重建 watch
// 采用指数退避（minRetryDelay ~ maxRetryDelay 封顶）持续重试，不会因瞬时故障销毁 watcher；
// 重连成功后重播全量服务数据，并从 Get 返回的 revision+1 处重建 watch 避免事件丢失；
// 仅当 watcherMgr 已停止或上下文结束时返回 false
// @param delay time.Duration 本次重试的等待间隔
// @return @1 bool 是否重连成功
// @return @2 time.Duration 后续重试应使用的等待间隔（全量同步连续失败时按指数增长，minRetryDelay ~ maxRetryDelay 封顶）
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

		// 从 Get 返回的 revision+1 开始重建 watch，避免事件丢失
		wm.watchChan = wm.watcher.Watch(
			wm.ctx,
			wm.watchKey,
			clientv3.WithPrefix(),
			clientv3.WithRev(res.Header.Revision+1),
		)

		return true, delay
	}
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
// 对缓存的实例做深拷贝后返回，避免调用方修改污染缓存数据
// @return @1 []*registry.ServiceInstance 服务实例列表
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
// 管理器已停止时返回错误
// @return @1 []*registry.ServiceInstance 服务实例列表
// @return @2 error 管理器已停止时返回的错误
func (wm *watcherMgr) services() ([]*registry.ServiceInstance, error) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	if wm.stopped.Load() {
		return nil, errors.ErrWatcherStopped
	}

	return wm.loadServices(), nil
}
