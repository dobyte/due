package core

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/dobyte/due/v2/config"
	"github.com/fsnotify/fsnotify"
)

type watcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	source  *Source
	watcher *fsnotify.Watcher
	stopped atomic.Bool
}

func newWatcher(ctx context.Context, source *Source) (*watcher, error) {
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

			// Dot files are not watched; a dot directory has to be skipped with fs.SkipDir instead
			// of returning nil, which would keep walking into it.
			if path != source.path && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}

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

// Next blocks until a config changes and returns the changed configurations.
func (w *watcher) Next() ([]*config.Configuration, error) {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil, io.EOF
			}

			// Ignore dot files and other temporary files, matching the filter used when loading a
			// config directory.
			if strings.HasPrefix(filepath.Base(event.Name), ".") {
				continue
			}

			// A newly created directory must be added to the watcher, otherwise changes to its
			// config files cannot be observed. Files that already exist in the directory are
			// loaded immediately, because a non-recursive watcher only observes events raised
			// after the directory is added.
			if event.Has(fsnotify.Create) {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					if addErr := w.addDir(event.Name); addErr != nil {
						return nil, addErr
					}

					configs, loadErr := w.source.loadDir(w.ctx, event.Name)
					if loadErr != nil {
						return nil, loadErr
					}

					if len(configs) > 0 {
						return configs, nil
					}

					continue
				}
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				if !w.source.matchFile(event.Name) {
					continue
				}

				// A rename event is also raised for the old name, which no longer exists, so there
				// is nothing to load; a rename onto this name is handled as a reload.
				if info, statErr := os.Stat(event.Name); statErr != nil || info.IsDir() {
					continue
				}

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

// addDir adds dir and its subdirectories to the watcher, matching the walk performed when the
// watcher starts.
func (w *watcher) addDir(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path != dir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}

			return nil
		}

		return w.watcher.Add(path)
	})
}

// Stop stops watching.
func (w *watcher) Stop() error {
	if !w.stopped.CompareAndSwap(false, true) {
		return nil
	}

	w.cancel()
	w.source.watchers.Delete(w)

	return w.watcher.Close()
}
