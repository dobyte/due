package polaris

import (
	"context"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
)

// watcher 监听器
type watcher struct {
	ctx     context.Context              // 上下文
	cancel  context.CancelFunc           // 取消函数
	source  *Source                      // 配置源
	chWatch chan []*config.Configuration // 配置变更通知通道
}

// newWatcher 创建监听器
// @param ctx context.Context 上下文
// @param s *Source 配置源
// @return @1 *watcher 监听器
// @return @2 error 错误信息
func newWatcher(ctx context.Context, s *Source) (*watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan []*config.Configuration, 2)

	return w, nil
}

// notice 通知配置变更
// 丢弃旧的未消费数据，保证只发送最新的配置，并以非阻塞方式发送，避免阻塞通知流程
// @param configuration *config.Configuration 变更后的配置项
func (w *watcher) notice(configuration *config.Configuration) {
	// 丢弃旧数据，保证只发送最新的配置
	for {
		select {
		case <-w.chWatch:
		default:
			goto SEND
		}
	}

SEND:
	// 非阻塞发送，避免消费缓慢或已停止的监听器阻塞通知流程
	select {
	case w.chWatch <- []*config.Configuration{configuration}:
	default:
	}
}

// Next 获取变更后的配置列表
// 阻塞等待配置变更通知，上下文取消时返回错误
// @return @1 []*config.Configuration 配置列表
// @return @2 error 错误信息
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

// Stop 停止监听
// 监听由Source统一管理，停止单个watcher时无需取消配置监听，
// 配置的监听与取消由Source内部的search与listen循环负责
// @return @1 error 错误信息
func (w *watcher) Stop() error {
	w.cancel()
	w.source.watchers.Delete(w)

	return nil
}
