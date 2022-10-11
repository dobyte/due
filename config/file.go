package config

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type defaultSource struct {
	path string
}

func NewSource(path string) Source {
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
		Name:    strings.TrimRight(info.Name(), ext),
		Format:  strings.TrimLeft(ext, "."),
		Content: content,
	}, nil
}

// 加载目录配置
func (s *defaultSource) loadDir(path string) (configurations []*Configuration, err error) {
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		configuration, err := s.loadFile(path)
		if err != nil {
			return err
		}
		configurations = append(configurations, configuration)

		return nil
	})

	return
}
