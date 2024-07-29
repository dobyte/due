package nacos

import (
	"context"
	"github.com/dobyte/due/v2/config"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type chWatch struct {
	key     string
	content string
}

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
	w.chWatch <- []*config.Configuration{configuration}
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
	return w.source.opts.client.CancelListenConfig(vo.ConfigParam{
		Group: w.source.opts.groupName,
	})
}
