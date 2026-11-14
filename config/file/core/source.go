package core

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/core/pattern"
	"github.com/dobyte/due/v2/errors"
)

// Name is the name of the file config source.
const Name = "file"

// Source is a file-based config source.
type Source struct {
	path     string
	mode     config.Mode
	files    []string    // Selected files; empty means every file under the source path
	watchers sync.Map    // Watcher set
	closed   atomic.Bool // Whether the source has been closed
}

var _ config.Source = &Source{}

// NewSource returns a file-based config source rooted at path with the given mode. The optional
// files select the files to load, store and watch, relative to path; an entry is an exact path
// unless it contains "*", which matches any sequence of characters. When files is empty, every
// file under the source path is available.
func NewSource(path string, mode config.Mode, files ...string) *Source {
	return &Source{path: strings.TrimSuffix(path, "/"), mode: mode, files: pattern.NormalizeFiles(files)}
}

// Name returns the name of the source.
func (s *Source) Name() string {
	return Name
}

// Load loads the configurations of the given files, or every file under the source path when file
// is empty.
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	explicit := len(file) > 0 && file[0] != ""
	path := s.path

	if explicit {
		info, err := os.Stat(s.path)
		if err != nil {
			return nil, err
		}

		if !info.IsDir() {
			return nil, errors.New("the specified file cannot be loaded at the file path")
		}

		if err = checkFilePath(s.path, file[0]); err != nil {
			return nil, err
		}

		path = filepath.Join(s.path, file[0])
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// A directory is expanded and every file in it is filtered by the configured files, so the
	// directory path itself must not be checked against the list.
	if info.IsDir() {
		return s.loadDir(path)
	}

	if !s.matchFile(path) {
		if explicit {
			return nil, errors.ErrFileNotConfigured
		}

		return nil, nil
	}

	c, err := s.loadFile(path)
	if err != nil {
		return nil, err
	}

	return []*config.Configuration{c}, nil
}

// Store saves content as file under the source path.
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.mode != config.WriteOnly && s.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	info, err := os.Stat(s.path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return errors.New("the specified file cannot be modified under the file path")
	}

	if err = checkFilePath(s.path, file); err != nil {
		return err
	}

	if !s.matchFile(filepath.Join(s.path, file)) {
		return errors.ErrFileNotConfigured
	}

	return writeFileAtomic(filepath.Join(s.path, file), content)
}

// checkFilePath validates that file is a safe path under root; it rejects absolute paths and paths
// containing "..".
func checkFilePath(root, file string) error {
	if file == "" {
		return errors.New("invalid file path: empty")
	}

	if filepath.IsAbs(file) {
		return errors.New("invalid file path: absolute path is not allowed")
	}

	for _, item := range strings.Split(filepath.ToSlash(file), "/") {
		if item == ".." {
			return errors.New("invalid file path: directory traversal is not allowed")
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	absTarget, err := filepath.Abs(filepath.Join(root, file))
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("invalid file path: directory traversal is not allowed")
	}

	return nil
}

// matchFile reports whether path is selected by the configured files. When no file is configured,
// every path is selected.
func (s *Source) matchFile(path string) bool {
	if len(s.files) == 0 {
		return true
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	absRoot, err := filepath.Abs(s.path)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	// In single-file mode the relative path is the current directory, so use the file name.
	if rel == "." {
		rel = filepath.Base(absPath)
	}

	return pattern.MatchFile(s.files, filepath.ToSlash(rel))
}

// Watch starts watching the source path and returns a [config.Watcher]. It reports
// [errors.ErrClosedConfigSource] once the source has been closed.
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.closed.Load() {
		return nil, errors.ErrClosedConfigSource
	}

	w, err := newWatcher(ctx, s)
	if err != nil {
		return nil, err
	}

	s.watchers.Store(w, struct{}{})

	// The source may have been closed between the check above and the registration, in which case
	// the shutdown no longer visits the watcher. Stop it so that it does not stay registered.
	if s.closed.Load() {
		_ = w.Stop()

		return nil, errors.ErrClosedConfigSource
	}

	return w, nil
}

// Close closes the source. It stops the registered watchers.
func (s *Source) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	s.watchers.Range(func(key, _ any) bool {
		_ = key.(*watcher).Stop()

		return true
	})

	return nil
}

// loadFile loads a single config file.
func (s *Source) loadFile(path string) (*config.Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(info.Name())

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	absRoot, err := filepath.Abs(s.path)
	if err != nil {
		return nil, err
	}

	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return nil, err
	}

	// In single-file mode the relative path is the current directory, so use the file name as the path.
	fullPath := filepath.Join(s.path, rel)
	if rel == "." {
		rel = info.Name()
		fullPath = s.path
	}

	return &config.Configuration{
		Path:     rel,
		File:     info.Name(),
		Name:     strings.TrimSuffix(info.Name(), ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  content,
		FullPath: fullPath,
	}, nil
}

// loadDir loads every config file under path.
func (s *Source) loadDir(path string) (cs []*config.Configuration, err error) {
	root := path

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Returning nil does not skip the directory, so a dot directory has to be skipped with
			// fs.SkipDir; otherwise the files inside it would be loaded.
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}

			return nil
		}

		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		if !s.matchFile(path) {
			return nil
		}

		c, err := s.loadFile(path)
		if err != nil {
			return err
		}
		cs = append(cs, c)

		return nil
	})

	return
}

// writeFileAtomic writes content to path through a temporary file and renames it into place, so
// that a concurrent watcher never reads a partially written config file. The temporary file name
// starts with a dot, so it is ignored by both the directory loader and the watcher.
func writeFileAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	file, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}

	name := file.Name()

	if _, err = file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(name)

		return err
	}

	if err = file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(name)

		return err
	}

	if err = file.Close(); err != nil {
		_ = os.Remove(name)

		return err
	}

	if err = os.Chmod(name, 0o644); err != nil {
		_ = os.Remove(name)

		return err
	}

	if err = os.Rename(name, path); err != nil {
		_ = os.Remove(name)

		return err
	}

	return nil
}
