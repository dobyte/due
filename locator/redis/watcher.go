package redis

import (
	"context"
	"fmt"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/locator"
	"github.com/dobyte/due/log"
	"github.com/go-redis/redis/v8"
	"sync"
	"sync/atomic"
)

type watcher struct {
	idx        int64
	state      int32
	ctx        context.Context
	cancel     context.CancelFunc
	chEvent    chan []*locator.Event
	watcherMgr *watcherMgr
}

func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.idx = idx
	w.watcherMgr = wm
	w.ctx, w.cancel = context.WithCancel(wm.ctx)
	w.chEvent = make(chan []*locator.Event, 16)

	return w
}

func (w *watcher) notify(events []*locator.Event) {
	if atomic.LoadInt32(&w.state) == 0 {
		return
	}

	w.chEvent <- events
}

// Next 返回变动事件列表
func (w *watcher) Next() ([]*locator.Event, error) {
	if atomic.LoadInt32(&w.state) == 0 {
		atomic.StoreInt32(&w.state, 1)
	}

	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case events, ok := <-w.chEvent:
		if !ok {
			if err := w.ctx.Err(); err != nil {
				return nil, err
			}
		}

		return events, nil
	}
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	close(w.chEvent)
	return w.watcherMgr.recycle(w.idx)
}

type watcherMgr struct {
	ctx     context.Context
	cancel  context.CancelFunc
	locator *Locator
	key     string
	sub     *redis.PubSub

	rw       sync.RWMutex
	idx      int64
	watchers map[int64]*watcher
}

func newWatcherMgr(ctx context.Context, l *Locator, key string, insKinds ...cluster.Kind) (*watcherMgr, error) {
	sub := l.opts.client.Subscribe(ctx)
	channels := make([]string, 0, len(insKinds))
	for _, insKind := range insKinds {
		channels = append(channels, fmt.Sprintf(channelEventKey, insKind.String()))
	}

	err := sub.Subscribe(ctx, channels...)
	if err != nil {
		return nil, err
	}

	wm := &watcherMgr{}
	wm.ctx, wm.cancel = context.WithCancel(l.ctx)
	wm.locator = l
	wm.watchers = make(map[int64]*watcher)
	wm.key = key
	wm.sub = sub

	go func() {
		for {
			iface, err := wm.sub.Receive(wm.ctx)
			if err != nil {
				return
			}

			switch v := iface.(type) {
			case *redis.Subscription:
				log.Debugf("channel subscribe succeeded, %s", v.Channel)
			case *redis.Message:
				event, err := unmarshal([]byte(v.Payload))
				if err != nil {
					log.Errorf("invalid payload, %s", v.Payload)
					continue
				}
				wm.broadcast(event)
			}
		}
	}()

	return wm, nil
}

func (wm *watcherMgr) fork() locator.Watcher {
	wm.rw.Lock()
	defer wm.rw.Unlock()

	w := newWatcher(wm, atomic.AddInt64(&wm.idx, 1))
	wm.watchers[w.idx] = w

	return w
}

func (wm *watcherMgr) recycle(idx int64) error {
	wm.rw.Lock()
	defer wm.rw.Unlock()

	delete(wm.watchers, idx)

	if len(wm.watchers) == 0 {
		wm.cancel()
		wm.locator.watchers.Delete(wm.key)
		return wm.sub.Close()
	}

	return nil
}

func (wm *watcherMgr) broadcast(events ...*locator.Event) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	for _, w := range wm.watchers {
		w.notify(events)
	}
}
