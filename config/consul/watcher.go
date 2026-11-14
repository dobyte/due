package consul

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

const (
	watchMinBackoff = time.Second      // Minimum backoff for watch reconnection
	watchMaxBackoff = 30 * time.Second // Maximum backoff for watch reconnection
)

// watcher watches the configuration changes of a Consul config source.
type watcher struct {
	ctx      context.Context                  // Context
	cancel   context.CancelFunc               // Cancel function
	source   *Source                          // Config source
	planMu   sync.Mutex                       // Guards the current watch plan
	plan     *watch.Plan                      // Current watch plan
	mu       sync.Mutex                       // Send lock
	configMu sync.Mutex                       // Guards the configuration snapshot
	configs  map[string]*config.Configuration // Configuration snapshot
	stopped  atomic.Bool                      // Whether the watcher has stopped
	warned   atomic.Bool                      // Whether the current watch failure was logged
	chWatch  chan []*config.Configuration     // Configuration change channel
	wg       sync.WaitGroup                   // Waits for the watch goroutine to exit
}

// newWatcher creates a watcher.
func newWatcher(ctx context.Context, s *Source) (*watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan []*config.Configuration, 2)
	w.configs = make(map[string]*config.Configuration)

	// Parse the plan once so that an invalid plan is reported to the caller instead of being retried
	// in the background. The parsed plan is discarded, because every run attempt owns a fresh plan.
	if _, err := w.newPlan(); err != nil {
		return nil, err
	}

	w.wg.Go(w.run)

	return w, nil
}

// newPlan creates a watch plan for the config prefix. A stopped plan cannot be run again, so every
// run attempt has to create a new one.
func (w *watcher) newPlan() (*watch.Plan, error) {
	plan, err := watch.Parse(map[string]any{
		"type":   "keyprefix",
		"prefix": w.source.opts.path + "/",
	})
	if err != nil {
		return nil, err
	}

	plan.Handler = w.planHandler

	return plan, nil
}

// run keeps the watch plan running until the watcher is stopped. Every failed attempt is retried
// with a backoff and a fresh plan.
func (w *watcher) run() {
	delay := watchMinBackoff

	for {
		if w.ctx.Err() != nil {
			return
		}

		plan, err := w.newPlan()
		if err != nil {
			if w.warned.CompareAndSwap(false, true) {
				log.Warnf("consul watch failed: %v", err)
			}

			if !w.sleep(delay) {
				return
			}

			delay = nextWatchDelay(delay)

			continue
		}

		w.setPlan(plan)

		// Stop may have run before the plan was registered, in which case the new plan must be
		// stopped instead of being run.
		if w.ctx.Err() != nil {
			w.setPlan(nil)
			plan.Stop()

			return
		}

		runErr := plan.RunWithClientAndHclog(w.source.opts.client, nil)
		w.setPlan(nil)

		if w.ctx.Err() != nil {
			return
		}

		if runErr != nil {
			if w.warned.CompareAndSwap(false, true) {
				log.Warnf("consul watch failed: %v", runErr)
			}

			if !w.sleep(delay) {
				return
			}

			delay = nextWatchDelay(delay)

			continue
		}

		return
	}
}

// sleep waits for delay or until the watcher is stopped. It reports false when the watcher is
// stopped.
func (w *watcher) sleep(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-w.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// nextWatchDelay doubles delay and caps it at watchMaxBackoff.
func nextWatchDelay(delay time.Duration) time.Duration {
	if delay >= watchMaxBackoff {
		return watchMaxBackoff
	}

	if delay *= 2; delay > watchMaxBackoff {
		delay = watchMaxBackoff
	}

	return delay
}

// setPlan records the plan that is currently running.
func (w *watcher) setPlan(plan *watch.Plan) {
	w.planMu.Lock()
	w.plan = plan
	w.planMu.Unlock()
}

// currentPlan returns the plan that is currently running, or nil when no plan is running.
func (w *watcher) currentPlan() *watch.Plan {
	w.planMu.Lock()
	defer w.planMu.Unlock()

	return w.plan
}

// planHandler handles the callback of the watch plan. It converts the KV set returned by Consul into
// a configuration list, updates the snapshot and reports only the new or changed configs.
func (w *watcher) planHandler(idx uint64, raw any) {
	if raw == nil {
		return // ignore
	}

	kvs, ok := raw.(api.KVPairs)
	if !ok {
		return
	}

	if w.warned.CompareAndSwap(true, false) {
		log.Infof("consul watch recovered")
	}

	configs := make([]*config.Configuration, 0, len(kvs))
	for _, kv := range kvs {
		c := w.source.parseKV(kv.Key, kv.Value)
		if !w.source.matchFile(c.Path) {
			continue
		}

		configs = append(configs, c)
	}

	if changed := w.diff(configs); len(changed) > 0 {
		w.notify(changed)
	}
}

// diff records configs as the current snapshot and returns the configs that are new or changed. A
// removed config is dropped from the snapshot but not reported, because a deletion has no
// representation in a config item and the value store is merge-only.
func (w *watcher) diff(configs []*config.Configuration) []*config.Configuration {
	w.configMu.Lock()
	defer w.configMu.Unlock()

	changed := make([]*config.Configuration, 0, len(configs))
	seen := make(map[string]struct{}, len(configs))

	for _, c := range configs {
		seen[c.FullPath] = struct{}{}

		prev, ok := w.configs[c.FullPath]
		if ok && bytes.Equal(prev.Content, c.Content) {
			continue
		}

		w.configs[c.FullPath] = c
		changed = append(changed, c)
	}

	for key := range w.configs {
		if _, ok := seen[key]; !ok {
			delete(w.configs, key)
		}
	}

	return changed
}

// notify notifies the watcher that configurations have changed. The change batch is a delta, so it
// is delivered as is: dropping or merging a batch would lose the changes it carries.
func (w *watcher) notify(configs []*config.Configuration) {
	if w.stopped.Load() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped.Load() {
		return
	}

	select {
	case w.chWatch <- configs:
	case <-w.ctx.Done():
	}
}

// Next returns the changed configuration list. It blocks until the configuration changes and returns
// an error once the watcher has been stopped.
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

	if plan := w.currentPlan(); plan != nil {
		plan.Stop()
	}

	// Wait for the watch goroutine to exit before closing the channel, so that no callback can send
	// to a closed channel.
	w.wg.Wait()

	w.mu.Lock()
	close(w.chWatch)
	w.mu.Unlock()

	w.source.watchers.Delete(w)

	return nil
}
