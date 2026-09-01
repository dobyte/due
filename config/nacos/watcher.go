package nacos

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/config"
)

type watcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	source  *Source
	chWatch chan struct{}
	mu      sync.Mutex
	pending map[string]*config.Configuration
}

func newWatcher(ctx context.Context, s *Source) *watcher {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan struct{}, 1)
	w.pending = make(map[string]*config.Configuration)

	return w
}

// notice 通知配置变更，按dataId合并，保证每个dataId的最新配置都会被送达
func (w *watcher) notice(configuration *config.Configuration) {
	w.mu.Lock()
	w.pending[configuration.File] = configuration
	w.mu.Unlock()

	// 非阻塞发送信号，避免消费缓慢或已停止的监听器阻塞通知流程
	select {
	case w.chWatch <- struct{}{}:
	default:
	}
}

// Next 返回配置列表
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

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	w.source.watchers.Delete(w)

	return nil
}
