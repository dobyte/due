package nacos

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/config"
)

type watcher struct {
	ctx     context.Context                  // 上下文
	cancel  context.CancelFunc               // 取消函数
	source  *Source                          // 配置源
	chWatch chan struct{}                    // 配置变更通知信号
	mu      sync.Mutex                       // 待投递配置互斥锁
	pending map[string]*config.Configuration // 待投递配置，以dataId为键合并最新配置
}

// newWatcher 创建监听器
// @param ctx context.Context 上下文
// @param s *Source 配置源
// @return @1 *watcher 监听器
func newWatcher(ctx context.Context, s *Source) *watcher {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan struct{}, 1)
	w.pending = make(map[string]*config.Configuration)

	return w
}

// notice 通知配置变更
// 按dataId合并待投递配置，保证每个dataId的最新配置都会被送达，
// 并以非阻塞方式发送信号，避免阻塞通知流程
// @param configuration *config.Configuration 变更后的配置项
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

// Next 获取变更后的配置列表
// 阻塞等待配置变更通知，返回当前所有待投递的最新配置；上下文取消时返回错误
// @return @1 []*config.Configuration 配置列表
// @return @2 error 错误信息
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
// 监听由Source统一管理，停止单个watcher时无需取消配置监听，
// 配置的监听与取消由Source内部的search与listen循环负责
// @return @1 error 错误信息
func (w *watcher) Stop() error {
	w.cancel()
	w.source.watchers.Delete(w)

	return nil
}
