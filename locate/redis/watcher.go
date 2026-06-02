package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/redis/go-redis/v9"
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
	chEvent    chan []*locate.Event
}

func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.idx = idx
	w.watcherMgr = wm
	w.ctx, w.cancel = context.WithCancel(wm.ctx)
	w.chEvent = make(chan []*locate.Event, 16)

	return w
}

func (w *watcher) notify(events []*locate.Event) {
	w.rw.RLock()
	defer w.rw.RUnlock()

	if w.state == stateRunning {
		w.chEvent <- events
	}
}

// Next 返回变动事件列表
func (w *watcher) Next() ([]*locate.Event, error) {
	w.rw.Lock()
	if w.state == stateInitial {
		w.state = stateRunning
	}
	w.rw.Unlock()

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
	w.rw.Lock()
	defer w.rw.Unlock()

	if w.state == stateStopped {
		return errors.ErrIllegalOperation
	}

	w.state = stateStopped
	w.cancel()
	close(w.chEvent)
	return w.watcherMgr.recycle(w.idx)
}

type watcherMgr struct {
	ctx      context.Context
	cancel   context.CancelFunc
	locator  *Locator
	key      string
	sub      *redis.PubSub
	rw       sync.RWMutex
	idx      int64
	watchers map[int64]*watcher
}

func newWatcherMgr(ctx context.Context, l *Locator, key string, kinds ...string) (*watcherMgr, error) {
	sub := l.opts.client.Subscribe(ctx)
	channels := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		channels = append(channels, fmt.Sprintf(clusterEventKey, l.opts.prefix, kind))
	}

	if err := sub.Subscribe(ctx, channels...); err != nil {
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

func (wm *watcherMgr) fork() locate.Watcher {
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

func (wm *watcherMgr) broadcast(events ...*locate.Event) {
	wm.rw.RLock()
	defer wm.rw.RUnlock()

	for _, w := range wm.watchers {
		w.notify(events)
	}
}
