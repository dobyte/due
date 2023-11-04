package consul

import (
	"github.com/dobyte/due/v2/config"
	"github.com/hashicorp/consul/api/watch"
)

type watcher struct {
}

func newWatcher(s *Source) (config.Watcher, error) {
	var prefix string

	if s.opts.path != "" {
		prefix = s.opts.path + "/"
	}

	plan, err := watch.Parse(map[string]interface{}{
		"type":   "keyprefix",
		"prefix": prefix,
	})
	if err != nil {
		return nil, err
	}

	plan.Handler = func(u uint64, i interface{}) {

	}

	err = plan.RunWithClientAndHclog(s.opts.client, nil)
	if err != nil {
		return nil, err
	}

	return &watcher{}, nil
}

// Next 返回配置列表
func (w *watcher) Next() ([]*config.Configuration, error) {
	return nil, nil
}

// Stop 停止监听
func (w *watcher) Stop() error {
	return nil
}
