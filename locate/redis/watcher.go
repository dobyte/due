package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/redis/go-redis/v9"
)

// 监听状态
type state int32

const (
	stateInitial state = 0 // 初始状态
	stateRunning state = 1 // 运行状态
	stateStopped state = 2 // 停止状态
)

// 定位监听器
type watcher struct {
	idx        int64                // 监听器序号
	ctx        context.Context      // 上下文
	cancel     context.CancelFunc   // 取消函数
	watcherMgr *watcherMgr          // 监听管理器
	rw         sync.RWMutex         // 读写锁
	state      state                // 监听状态
	chEvent    chan []*locate.Event // 事件通道
}

// 创建定位监听器
// @param wm *watcherMgr 监听管理器
// @param idx int64 监听器序号
// @return @1 *watcher 定位监听器
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.idx = idx
	w.watcherMgr = wm
	w.ctx, w.cancel = context.WithCancel(wm.ctx)
	w.chEvent = make(chan []*locate.Event, 1024)

	return w
}

// 通知监听器
// 将变动事件发送给监听器，事件通道未满时直接发送，已满时丢弃最旧事件后发送最新事件，避免阻塞广播协程
// @param events []*locate.Event 变动事件列表
func (w *watcher) notify(events []*locate.Event) {
	w.rw.RLock()
	defer w.rw.RUnlock()

	if w.state != stateRunning {
		return
	}

	// 通道未满时直接发送
	select {
	case w.chEvent <- events:
		return
	default:
	}

	// 通道已满时丢弃最旧事件
	select {
	case <-w.chEvent:
	default:
	}

	// 发送最新事件
	select {
	case w.chEvent <- events:
	default:
	}
}

// Next 返回变动事件列表
// @return @1 []*locate.Event 变动事件列表
// @return @2 error 监听停止或上下文取消时返回的错误
func (w *watcher) Next() ([]*locate.Event, error) {
	w.rw.Lock()
	if w.state == stateInitial {
		w.state = stateRunning
	}
	w.rw.Unlock()

	select {
	case <-w.ctx.Done():
		return nil, errors.ErrWatcherStopped
	case events, ok := <-w.chEvent:
		if !ok {
			return nil, errors.ErrWatcherStopped
		}

		return events, nil
	}
}

// Stop 停止监听
// @return @1 error 重复停止时返回的错误
func (w *watcher) Stop() error {
	w.rw.Lock()
	if w.state == stateStopped {
		w.rw.Unlock()
		return errors.ErrIllegalOperation
	}

	w.state = stateStopped
	w.cancel()
	close(w.chEvent)
	w.rw.Unlock()

	w.watcherMgr.recycle(w.idx)

	return nil
}

// 定位监听管理器
type watcherMgr struct {
	ctx      context.Context    // 上下文
	cancel   context.CancelFunc // 取消函数
	locator  *Locator           // 定位器
	key      string             // 唯一键
	channels []string           // 订阅频道
	rw       sync.RWMutex       // 读写锁
	wg       sync.WaitGroup     // 接收协程等待组
	idx      atomic.Int64       // 监听器序号
	stopped  atomic.Bool        // 停止标志
	watchers map[int64]*watcher // 监听器集合
}

// 创建定位监听管理器
// 订阅指定实例类型的事件通道，并启动协程消费发布订阅消息
// @param l *Locator 定位器
// @param key string 唯一键
// @param kinds ...string 实例类型列表
// @return @1 *watcherMgr 定位监听管理器
// @return @2 error 订阅失败时返回的错误
func newWatcherMgr(l *Locator, key string, kinds ...string) (*watcherMgr, error) {
	if len(kinds) == 0 {
		return nil, errors.ErrInvalidArgument
	}

	channels := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		channels = append(channels, fmt.Sprintf(clusterEventKey, l.opts.prefix, l.opts.db, kind))
	}

	wm := &watcherMgr{}
	wm.ctx, wm.cancel = context.WithCancel(l.ctx)
	wm.locator = l
	wm.watchers = make(map[int64]*watcher)
	wm.key = key
	wm.channels = channels

	sub, err := wm.subscribe()
	if err != nil {
		return nil, err
	}

	wm.wg.Go(func() {
		wm.watch(sub)
	})

	return wm, nil
}

// 订阅事件频道
// 创建Redis发布订阅并订阅全部事件频道
// @return @1 *redis.PubSub Redis发布订阅
// @return @2 error 订阅失败时返回的错误
func (wm *watcherMgr) subscribe() (*redis.PubSub, error) {
	sub := wm.locator.opts.client.Subscribe(wm.ctx)

	if err := sub.Subscribe(wm.ctx, wm.channels...); err != nil {
		_ = sub.Close()
		return nil, err
	}

	return sub, nil
}

// 消费发布订阅消息
// 持续接收并广播发布订阅消息，连接异常时退避重连并重新订阅
// @param sub *redis.PubSub Redis发布订阅
func (wm *watcherMgr) watch(sub *redis.PubSub) {
	defer func() {
		if sub != nil {
			_ = sub.Close()
		}
	}()

	for {
		iface, err := sub.Receive(wm.ctx)
		if err != nil {
			if wm.ctx.Err() != nil {
				return
			}

			if !errors.Is(err, redis.ErrClosed) {
				log.Errorf("receive pubsub message failed: %v", err)
			}

			var ok bool
			if sub, ok = wm.resubscribe(sub); !ok {
				wm.shutdown()
				return
			}
			continue
		}

		switch v := iface.(type) {
		case *redis.Message:
			event, err := unmarshal([]byte(v.Payload))
			if err != nil {
				log.Errorf("invalid payload, %s", v.Payload)
				continue
			}
			wm.broadcast(event)
		}
	}
}

// 重连并重新订阅
// 关闭旧订阅，以指数退避重试重新订阅事件频道
// @param old *redis.PubSub 旧发布订阅
// @return @1 *redis.PubSub 新发布订阅
// @return @2 bool 是否重连成功
func (wm *watcherMgr) resubscribe(old *redis.PubSub) (*redis.PubSub, bool) {
	_ = old.Close()

	var sub *redis.PubSub
	err := xcall.Backoff(wm.ctx, func(ctx context.Context, attempt int) (bool, error) {
		sub = wm.locator.opts.client.Subscribe(ctx)
		if err := sub.Subscribe(ctx, wm.channels...); err != nil {
			_ = sub.Close()
			log.Warnf("resubscribe failed, retry %d times, err: %v", attempt, err)
			return true, err
		}

		return false, nil
	}, defaultRetryTimes, 100*time.Millisecond, 3*time.Second)

	if err != nil {
		return nil, false
	}

	return sub, true
}

// 派生监听器
// 从监听管理器派生一个新的监听器
// @return @1 locate.Watcher 定位监听器
// @return @2 error 监听管理器已关闭时返回的错误
func (wm *watcherMgr) fork() (locate.Watcher, error) {
	wm.rw.Lock()
	defer wm.rw.Unlock()

	if wm.stopped.Load() || wm.ctx.Err() != nil {
		return nil, errors.ErrWatcherStopped
	}

	w := newWatcher(wm, wm.idx.Add(1))
	wm.watchers[w.idx] = w

	return w, nil
}

// 关闭监听管理器
// 重连彻底失败时由接收协程调用，删除缓存、停止所有监听器并取消上下文
func (wm *watcherMgr) shutdown() {
	wm.rw.Lock()
	watchers := make([]*watcher, 0, len(wm.watchers))
	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}

	if !wm.stopped.CompareAndSwap(false, true) {
		wm.rw.Unlock()
		return
	}

	wm.locator.watchers.Delete(wm.key)
	wm.rw.Unlock()

	for _, w := range watchers {
		w.Stop()
	}

	wm.cancel()
}

// 回收监听器
// 从监听管理器中移除指定监听器，无监听器时关闭监听管理器
// @param idx int64 监听器序号
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

	wm.locator.watchers.Delete(wm.key)
	wm.rw.Unlock()

	wm.cancel()
	wm.wg.Wait()
}

// 广播事件
// 将变动事件广播给所有监听器
// @param events ...*locate.Event 变动事件列表
func (wm *watcherMgr) broadcast(events ...*locate.Event) {
	wm.rw.RLock()
	watchers := make([]*watcher, 0, len(wm.watchers))
	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(events)
	}
}
