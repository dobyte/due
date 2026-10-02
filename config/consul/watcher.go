package consul

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

const (
	watchMinBackoff = time.Second      // Minimum backoff for watch reconnection
	watchMaxBackoff = 30 * time.Second // Maximum backoff for watch reconnection
)

// watcher watches the configuration changes of a Consul config source.
type watcher struct {
	ctx     context.Context              // Context
	cancel  context.CancelFunc           // Cancel function
	source  *Source                      // Config source
	plan    *watch.Plan                  // Watch plan
	mu      sync.Mutex                   // Send lock
	stopped atomic.Bool                  // Whether the watcher has stopped
	chWatch chan []*config.Configuration // Configuration change channel
}

// newWatcher creates a watcher.
func newWatcher(ctx context.Context, s *Source) (config.Watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan []*config.Configuration, 2)

	if err := w.init(); err != nil {
		return nil, err
	}

	return w, nil
}

// init initializes the watcher. It parses the plan that watches the key prefix and
// starts the watch goroutine.
func (w *watcher) init() (err error) {
	w.plan, err = watch.Parse(map[string]any{
		"type":   "keyprefix",
		"prefix": w.source.opts.path + "/",
	})
	if err != nil {
		return
	}

	w.plan.Handler = w.planHandler

	xcall.Go(func() {
		delay := watchMinBackoff

		for {
			if runErr := w.plan.RunWithClientAndHclog(w.source.opts.client, nil); runErr != nil {
				if w.ctx.Err() != nil {
					return
				}

				log.Warnf("consul watch failed: %v", runErr)

				select {
				case <-w.ctx.Done():
					return
				case <-time.After(delay):
				}

				if delay < watchMaxBackoff {
					if delay > watchMaxBackoff/2 {
						delay = watchMaxBackoff
					} else {
						delay *= 2
					}
				}

				continue
			}

			return
		}
	})

	return
}

// planHandler handles the callback of the watch plan. It converts the KV set
// returned by Consul into a configuration list and notifies the watcher.
func (w *watcher) planHandler(idx uint64, raw any) {
	if raw == nil {
		return // ignore
	}

	kvs, ok := raw.(api.KVPairs)
	if !ok {
		return
	}

	configs := make([]*config.Configuration, 0, len(kvs))
	for _, kv := range kvs {
		configs = append(configs, w.source.parseKV(kv.Key, kv.Value))
	}

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
		default:
			return
		}
	}
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

// Stop stops the watcher.
func (w *watcher) Stop() error {
	if !w.stopped.CompareAndSwap(false, true) {
		return nil
	}

	w.cancel()
	w.plan.Stop()

	w.mu.Lock()
	close(w.chWatch)
	w.mu.Unlock()

	return nil
}
