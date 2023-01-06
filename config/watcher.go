package config

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Watcher interface {
	// Next 返回服务实例列表
	Next() ([]*Configuration, error)
	// Stop 停止监听
	Stop() error
}

type defaultWatcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	source  *defaultSource
	watcher *fsnotify.Watcher
}

func newWatcher(ctx context.Context, source *defaultSource) (Watcher, error) {
	info, err := os.Stat(source.path)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
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

			return watcher.Add(path)
		})
	} else {
		err = watcher.Add(source.path)
	}
	if err != nil {
		return nil, err
	}

	w := &defaultWatcher{}
	w.source = source
	w.watcher = watcher
	w.ctx, w.cancel = context.WithCancel(ctx)

	return w, nil
}

// Next 返回服务实例列表
func (w *defaultWatcher) Next() ([]*Configuration, error) {
	select {
	case event, ok := <-w.watcher.Events:
		if !ok {
			return nil, nil
		}
		if event.Has(fsnotify.Write) {
			c, err := w.source.loadFile(event.Name)
			if err != nil {
				return nil, err
			}
			return []*Configuration{c}, nil
		}
	case err := <-w.watcher.Errors:
		return nil, err
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	}

	return nil, nil
}

// Stop 停止监听
func (w *defaultWatcher) Stop() error {
	w.cancel()
	return w.watcher.Close()
}
