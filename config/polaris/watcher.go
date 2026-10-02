package polaris

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/config"
)

// watcher watches configuration changes.
type watcher struct {
	ctx     context.Context                  // Context
	cancel  context.CancelFunc               // Cancel function
	source  *Source                          // Config source
	chWatch chan struct{}                    // Config change notification signal
	mu      sync.Mutex                       // Mutex guarding the pending configurations
	pending map[string]*config.Configuration // Pending configurations, keyed by file name to merge the latest one
}

// newWatcher creates a watcher.
func newWatcher(ctx context.Context, s *Source) (*watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan struct{}, 1)
	w.pending = make(map[string]*config.Configuration)

	return w, nil
}

// notice notifies a configuration change. It merges pending configurations by
// file name so that the latest configuration of every file is delivered, and sends
// the signal without blocking so that the notification flow is not stalled.
func (w *watcher) notice(configuration *config.Configuration) {
	w.mu.Lock()
	w.pending[configuration.File] = configuration
	w.mu.Unlock()

	// Send the signal without blocking so that a slow or stopped watcher does not stall notification.
	select {
	case w.chWatch <- struct{}{}:
	default:
	}
}

// Next returns the changed configuration list.
//
// It blocks until a configuration change notification arrives and returns every
// latest pending configuration; it returns an error when the context is cancelled.
func (w *watcher) Next() ([]*config.Configuration, error) {
	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case <-w.chWatch:
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pending) == 0 {
		return nil, nil
	}

	configs := make([]*config.Configuration, 0, len(w.pending))
	for _, configuration := range w.pending {
		configs = append(configs, configuration)
	}
	w.pending = make(map[string]*config.Configuration)

	return configs, nil
}

// Stop stops watching.
//
// Watching is managed by [Source], so stopping a single watcher does not need to
// cancel config listening; the [Source]'s internal search and listen loops are
// responsible for registering and cancelling listening.
func (w *watcher) Stop() error {
	w.cancel()
	w.source.watchers.Delete(w)

	return nil
}
