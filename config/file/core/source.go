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
	"github.com/dobyte/due/v2/utils/xfile"
)

const Name = "file"

type Source struct {
	path string
	mode config.Mode
}

var _ config.Source = &Source{}

func NewSource(path string, mode config.Mode) *Source {
	return &Source{path: strings.TrimSuffix(path, "/"), mode: mode}
}

// Name 配置源名称
func (s *Source) Name() string {
	return Name
}

// Load 加载配置
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

// Store 保存配置项
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

	return xfile.WriteFile(filepath.Join(s.path, file), content)
}

// Watch 监听配置变化
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	return newWatcher(ctx, s)
}

func (s *Source) Close() error {
	return nil
}

// 加载文件配置
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
	path1, _ := filepath.Abs(path)
	path2, _ := filepath.Abs(s.path)
	path = strings.TrimPrefix(path1, path2)
	fullPath := s.path + path

	return &config.Configuration{
		Path:     path,
		File:     info.Name(),
		Name:     strings.TrimSuffix(info.Name(), ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  content,
		FullPath: fullPath,
	}, nil
}

// 加载目录配置
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
