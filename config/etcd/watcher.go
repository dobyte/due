package etcd

import (
	"context"
	"github.com/dobyte/due/v2/config"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/v3"
	"path/filepath"
	"strings"
)

type watcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	source  *Source
	watcher clientv3.Watcher
	chWatch clientv3.WatchChan
}

func newWatcher(ctx context.Context, s *Source) (config.Watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.watcher = clientv3.NewWatcher(w.source.opts.client)
	w.chWatch = w.watcher.Watch(w.ctx, w.source.opts.path, clientv3.WithPrefix())

	return w, nil
}

// Next 返回服务实例列表
func (w *watcher) Next() ([]*config.Configuration, error) {
	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case res, ok := <-w.chWatch:
		if !ok {
			if err := w.ctx.Err(); err != nil {
				return nil, err
			}
		}

		if res.Err() != nil {
			return nil, res.Err()
		}

		configs := make([]*config.Configuration, 0, len(res.Events))
		for _, ev := range res.Events {
			switch ev.Type {
			case mvccpb.PUT:
				fullPath := string(ev.Kv.Key)
				path := strings.TrimPrefix(fullPath, w.source.opts.path)
				file := filepath.Base(fullPath)
				ext := filepath.Ext(file)
				configs = append(configs, &config.Configuration{
					Path:     path,
					File:     file,
					Name:     strings.TrimSuffix(file, ext),
					Format:   strings.TrimPrefix(ext, "."),
					Content:  ev.Kv.Value,
					FullPath: fullPath,
				})
			}
		}

		return configs, nil
	}
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	return w.watcher.Close()
}
