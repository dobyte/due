package config

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Source interface {
	// Load 加载配置项
	Load() ([]*Configuration, error)
	// Watch 监听配置项
	Watch(ctx context.Context) (Watcher, error)
	// Path 配置项路径
	Path() string
}

// Configuration 配置项
type Configuration struct {
	Name    string
	Format  string
	Content []byte
}

type defaultSource struct {
	path string
}

var _ Source = &defaultSource{}

func NewSource(path string) Source {
	if path == "" {
		return nil
	}

	return &defaultSource{path: path}
}

// Load 加载配置
func (s *defaultSource) Load() ([]*Configuration, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return s.loadDir(s.path)
	}

	c, err := s.loadFile(s.path)
	if err != nil {
		return nil, err
	}

	return []*Configuration{c}, nil
}

// Watch 监听配置变化
func (s *defaultSource) Watch(ctx context.Context) (Watcher, error) {
	return newWatcher(ctx, s)
}

// 加载文件配置
func (s *defaultSource) loadFile(path string) (*Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(info.Name())

	return &Configuration{
		Name:    strings.TrimSuffix(info.Name(), ext),
		Format:  strings.TrimPrefix(ext, "."),
		Content: content,
	}, nil
}

// 加载目录配置
func (s *defaultSource) loadDir(path string) (cs []*Configuration, err error) {
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || strings.HasSuffix(d.Name(), ".") {
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

// Path 配置项路径
func (s *defaultSource) Path() string {
	return s.path
}
