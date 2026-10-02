package core

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xos"
)

// Name is the name of the file config source.
const Name = "file"

// Source is a file-based config source.
type Source struct {
	path string
	mode config.Mode
}

var _ config.Source = &Source{}

// NewSource returns a file-based config source rooted at path with the given mode.
func NewSource(path string, mode config.Mode) *Source {
	return &Source{path: strings.TrimSuffix(path, "/"), mode: mode}
}

// Name returns the name of the source.
func (s *Source) Name() string {
	return Name
}

// Load loads the configurations of the given files, or every file under the source path when file
// is empty.
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	path := s.path

	if len(file) > 0 && file[0] != "" {
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

	if info.IsDir() {
		return s.loadDir(path)
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

	return xos.WriteFile(filepath.Join(s.path, file), content)
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

// Watch starts watching the source path and returns a [config.Watcher].
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	return newWatcher(ctx, s)
}

// Close closes the source.
func (s *Source) Close() error {
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
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
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
