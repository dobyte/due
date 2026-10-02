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

// state is the watcher state.
type state int32

const (
	stateInitial state = 0 // Initial state
	stateRunning state = 1 // Running state
	stateStopped state = 2 // Stopped state
)

// watcher is a locate watcher.
type watcher struct {
	idx        int64              // Watcher index
	ctx        context.Context    // Context
	cancel     context.CancelFunc // Cancel function
	watcherMgr *watcherMgr        // Watch manager
	rw         sync.RWMutex       // Read-write lock
	state      state              // Watcher state
	chEvent    chan *locate.Event // Event channel
}

// newWatcher creates a locate watcher.
func newWatcher(wm *watcherMgr, idx int64) *watcher {
	w := &watcher{}
	w.idx = idx
	w.watcherMgr = wm
	w.ctx, w.cancel = context.WithCancel(wm.ctx)
	w.chEvent = make(chan *locate.Event, 1024)

	return w
}

// notify notifies the watcher.
//
// It sends a change event to the watcher: when the event channel is not full the event is sent
// directly, and when it is full the oldest event is dropped before the newest one is sent, so that
// the broadcast goroutine is never blocked.
func (w *watcher) notify(event *locate.Event) {
	w.rw.RLock()
	defer w.rw.RUnlock()

	if w.state != stateRunning {
		return
	}

	// Send directly when the channel is not full.
	select {
	case w.chEvent <- event:
		return
	default:
	}

	// Drop the oldest event when the channel is full.
	select {
	case <-w.chEvent:
	default:
	}

	// Send the newest event.
	select {
	case w.chEvent <- event:
	default:
	}
}

// Next returns the list of change events.
//
// It returns the list of change events and an error when the watcher is stopped or the context is
// canceled.
func (w *watcher) Next() ([]*locate.Event, error) {
	w.rw.Lock()
	if w.state == stateInitial {
		w.state = stateRunning
	}
	w.rw.Unlock()

	select {
	case <-w.ctx.Done():
		return nil, errors.ErrWatcherStopped
	case event, ok := <-w.chEvent:
		if !ok {
			return nil, errors.ErrWatcherStopped
		}

		return []*locate.Event{event}, nil
	}
}

// Stop stops the watcher.
//
// It returns an error when the watcher is stopped repeatedly.
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

// watcherMgr is a locate watch manager.
type watcherMgr struct {
	ctx      context.Context    // Context
	cancel   context.CancelFunc // Cancel function
	locator  *Locator           // Locator
	key      string             // Unique key
	channels []string           // Subscribed channels
	rw       sync.RWMutex       // Read-write lock
	wg       sync.WaitGroup     // Wait group of the receiving goroutine
	idx      atomic.Int64       // Watcher index
	stopped  atomic.Bool        // Stopped flag
	watchers map[int64]*watcher // Set of watchers
}

// newWatcherMgr creates a locate watch manager.
//
// It subscribes to the event channels of the given instance kinds and starts a goroutine that
// consumes publish/subscribe messages.
//
// It returns the watch manager and an error when subscribing fails.
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

// subscribe subscribes to the event channels.
//
// It creates a redis publish/subscribe connection and subscribes to every event channel.
//
// It returns the redis publish/subscribe handle and an error when subscribing fails.
func (wm *watcherMgr) subscribe() (*redis.PubSub, error) {
	sub := wm.locator.opts.client.Subscribe(wm.ctx)

	if err := sub.Subscribe(wm.ctx, wm.channels...); err != nil {
		_ = sub.Close()
		return nil, err
	}

	return sub, nil
}

// watch consumes publish/subscribe messages.
//
// It keeps receiving and broadcasting publish/subscribe messages, and reconnects with backoff and
// resubscribes when the connection fails.
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

// resubscribe reconnects and resubscribes.
//
// It closes the old subscription and retries subscribing to the event channels with exponential
// backoff.
//
// It returns the new publish/subscribe handle and whether the reconnect succeeded.
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

// fork derives a watcher.
//
// It derives a new watcher from the watch manager.
//
// It returns the locate watcher and an error when the watch manager has been closed.
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

// shutdown closes the watch manager.
//
// It is called by the receiving goroutine when the reconnect fails completely; it removes the
// cache, stops every watcher and cancels the context.
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

// recycle recycles a watcher.
//
// It removes the given watcher from the watch manager and closes the watch manager when no watcher
// remains.
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

// broadcast broadcasts an event.
//
// It broadcasts a change event to every watcher.
func (wm *watcherMgr) broadcast(event *locate.Event) {
	wm.rw.RLock()
	watchers := make([]*watcher, 0, len(wm.watchers))
	for _, w := range wm.watchers {
		watchers = append(watchers, w)
	}
	wm.rw.RUnlock()

	for _, w := range watchers {
		w.notify(event)
	}
}
