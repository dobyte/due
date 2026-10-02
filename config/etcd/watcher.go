package etcd

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// watcher watches the configuration changes of an etcd config source.
type watcher struct {
	ctx     context.Context                  // Context
	cancel  context.CancelFunc               // Cancel function
	source  *Source                          // Config source
	watcher clientv3.Watcher                 // etcd watcher
	watchCh clientv3.WatchChan               // etcd watch channel
	mu      sync.Mutex                       // Send lock
	chWatch chan []*config.Configuration     // Configuration change channel
	rw      sync.RWMutex                     // Read-write lock guarding the configuration snapshot
	configs map[string]*config.Configuration // Configuration snapshot
	stopped atomic.Bool                      // Whether the watcher has stopped
	wg      sync.WaitGroup                   // Waits for the goroutine to exit
}

// newWatcher creates a watcher. It uses the full pull result as the initial
// snapshot and starts watching after the revision recorded at pull time, so that
// no configuration change is lost.
func newWatcher(ctx context.Context, s *Source, res *clientv3.GetResponse) *watcher {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.watcher = clientv3.NewWatcher(w.source.opts.client)
	w.chWatch = make(chan []*config.Configuration, 2)
	w.configs = make(map[string]*config.Configuration)

	if res != nil {
		// Use the full pull result as the initial snapshot.
		for _, kv := range res.Kvs {
			c := w.source.parseKV(kv.Key, kv.Value)
			w.configs[c.FullPath] = c
		}

		// Start watching after the revision of the pull so that changes between the
		// pull and the watch are not lost.
		w.watchCh = w.watcher.Watch(
			w.ctx,
			w.source.opts.path,
			clientv3.WithPrefix(),
			clientv3.WithRev(res.Header.Revision+1),
		)
	}

	w.wg.Go(func() {
		for {
			if w.watchCh == nil {
				// When the initial snapshot pull failed, reconnect with a full pull and
				// rebuild the watch first so that the watcher does not start with an
				// incomplete snapshot.
				if !w.resync() {
					return
				}
				continue
			}

			w.watchLoop()

			if w.stopped.Load() {
				return
			}

			if !w.resync() {
				return
			}
		}
	})

	return w
}

// Next returns the configuration list. It blocks until the configuration changes
// and returns an error once the watcher has been stopped.
func (w *watcher) Next() ([]*config.Configuration, error) {
	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case configs, ok := <-w.chWatch:
		if !ok {
			return nil, errors.ErrWatcherStopped
		}

		return configs, nil
	}
}

// watchLoop is the watch event loop. It handles etcd watch events: PUT updates an
// entry of the configuration snapshot and DELETE removes it; the latest
// configuration is then broadcast.
func (w *watcher) watchLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case res, ok := <-w.watchCh:
			if !ok {
				return
			}

			if res.Err() != nil {
				log.Warnf("etcd watch error: %v", res.Err())
				return
			}

			w.rw.Lock()
			for _, ev := range res.Events {
				switch ev.Type {
				case mvccpb.PUT:
					c := w.source.parseKV(ev.Kv.Key, ev.Kv.Value)
					w.configs[c.FullPath] = c
				case mvccpb.DELETE:
					delete(w.configs, string(ev.Kv.Key))
				}
			}
			w.rw.Unlock()

			w.broadcast()
		}
	}
}

// resync reconnects with a full pull and retries.
//
// After the watch has failed, resync re-pulls the full configuration and rebuilds
// the watch until it succeeds or the watcher is stopped.
func (w *watcher) resync() bool {
	for {
		err := xcall.Backoff(w.ctx, func(ctx context.Context, attempt int) (bool, error) {
			if w.stopped.Load() {
				return false, errors.ErrWatcherStopped
			}

			tctx, tcancel := context.WithTimeout(ctx, w.source.opts.timeout)
			res, err := w.source.opts.client.Get(tctx, w.source.opts.path, clientv3.WithPrefix())
			tcancel()
			if err != nil {
				log.Warnf("etcd watch resync failed, retry %d times, err: %v", attempt, err)
				return true, err
			}

			w.rw.Lock()
			w.configs = make(map[string]*config.Configuration)
			for _, kv := range res.Kvs {
				c := w.source.parseKV(kv.Key, kv.Value)
				w.configs[c.FullPath] = c
			}
			w.rw.Unlock()

			w.broadcast()

			w.watchCh = w.watcher.Watch(
				w.ctx,
				w.source.opts.path,
				clientv3.WithPrefix(),
				clientv3.WithRev(res.Header.Revision+1),
			)

			return false, nil
		}, defaultRetryTimes, 100*time.Millisecond, 3*time.Second)
		if err == nil {
			return true
		}

		if w.ctx.Err() != nil || w.stopped.Load() {
			return false
		}
	}
}

// broadcast broadcasts the configuration list. It reads the full configuration
// from the snapshot and notifies the watcher.
func (w *watcher) broadcast() {
	w.rw.RLock()
	configs := w.snapshot()
	w.rw.RUnlock()

	w.notify(configs)
}

// notify notifies the watcher that the configuration list has been updated. It
// clears the stale data and then sends the latest configuration snapshot in a
// non-blocking way.
func (w *watcher) notify(configs []*config.Configuration) {
	if w.stopped.Load() {
		return
	}

	w.mu.Lock()

	if w.stopped.Load() {
		w.mu.Unlock()
		return
	}

	w.flush()

	select {
	case w.chWatch <- configs:
	case <-w.ctx.Done():
	}

	w.mu.Unlock()
}

// flush clears every stale item and keeps only the latest configuration snapshot.
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

// snapshot returns the current full configuration list.
func (w *watcher) snapshot() []*config.Configuration {
	configs := make([]*config.Configuration, 0, len(w.configs))
	for _, c := range w.configs {
		configs = append(configs, c)
	}

	return configs
}

// Stop stops the watcher.
func (w *watcher) Stop() error {
	w.release()

	w.wg.Wait()

	w.watcher.Close()

	return nil
}

// release releases the resources. It cancels the context and closes the
// configuration change channel.
func (w *watcher) release() {
	if !w.stopped.CompareAndSwap(false, true) {
		return
	}

	w.cancel()

	w.mu.Lock()
	close(w.chWatch)
	w.mu.Unlock()
}
