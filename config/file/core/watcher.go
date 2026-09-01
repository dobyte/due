package core

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/fsnotify/fsnotify"
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
		_ = fsWatcher.Close()
		return nil, err
	}

	w := &watcher{}
	w.source = source
	w.watcher = fsWatcher
	w.ctx, w.cancel = context.WithCancel(ctx)

	return w, nil
}

// Next 返回配置列表
func (w *watcher) Next() ([]*config.Configuration, error) {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil, io.EOF
			}

			// 忽略点文件等临时文件，与加载目录配置时的过滤规则保持一致
			if strings.HasPrefix(filepath.Base(event.Name), ".") {
				continue
			}

			// 新建目录需添加到监听器，否则其内部配置文件的变更无法被感知
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.watcher.Add(event.Name)
					continue
				}
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				c, err := w.source.loadFile(event.Name)
				if err != nil {
					return nil, err
				}
				return []*config.Configuration{c}, nil
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil, io.EOF
			}
			return nil, err
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		}
	}
}

// Stop 停止监听
func (w *watcher) Stop() error {
	w.cancel()
	return w.watcher.Close()
}
