package nacos

import (
	"context"

	"github.com/dobyte/due/v2/config"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type watcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	source  *Source
	chWatch chan []*config.Configuration
}

func newWatcher(ctx context.Context, s *Source) (*watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan []*config.Configuration, 2)

	return w, nil
}

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

func (w *watcher) Next() ([]*config.Configuration, error) {
	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case configs, ok := <-w.chWatch:
		if !ok {
			if err := w.ctx.Err(); err != nil {
				return nil, err
			}
		}

		return configs, nil
	}
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	w.source.watchers.Delete(w)

	return w.source.opts.client.CancelListenConfig(vo.ConfigParam{
		Group: w.source.opts.groupName,
	})
}
