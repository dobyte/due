package core

import (
	"context"
	"github.com/dobyte/due/v2/config"
	"github.com/fsnotify/fsnotify"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type watcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	source  *Source
	watcher *fsnotify.Watcher
}

func newWatcher(ctx context.Context, source *Source) (config.Watcher, error) {
	info, err := os.Stat(source.path)
	if err != nil {
		return nil, err
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		err = filepath.WalkDir(source.path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			return fsWatcher.Add(path)
		})
	} else {
		err = fsWatcher.Add(source.path)
	}
	if err != nil {
		return nil, err
	}

	w := &watcher{}
	w.source = source
	w.watcher = fsWatcher
	w.ctx, w.cancel = context.WithCancel(ctx)

	return w, nil
}

// Next 返回服务实例列表
func (w *watcher) Next() ([]*config.Configuration, error) {
	select {
	case event, ok := <-w.watcher.Events:
		if !ok {
			return nil, nil
		}
		if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
			c, err := w.source.loadFile(event.Name)
			if err != nil {
				return nil, err
			}
			return []*config.Configuration{c}, nil
		}
	case err := <-w.watcher.Errors:
		return nil, err
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	}

	return nil, nil
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	return w.watcher.Close()
}
