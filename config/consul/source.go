package consul

import (
	"context"
	"github.com/dobyte/due/v2/config"
	"github.com/hashicorp/consul/api"
	"path/filepath"
	"strings"
)

const Name = "consul"

type Source struct {
	err     error
	opts    *options
	builtin bool
}

func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o
	s.opts.path = strings.TrimSuffix(strings.TrimPrefix(s.opts.path, "/"), "/")

	if o.client == nil {
		c := api.DefaultConfig()
		if o.addr != "" {
			c.Address = o.addr
		}

		s.builtin = true
		s.opts.client, s.err = api.NewClient(c)
	}

	return s
}

// Name 配置源名称
func (s *Source) Name() string {
	return Name
}

// Load 加载配置项
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	var prefix string

	if s.opts.path != "" {
		if len(file) > 0 && file[0] != "" {
			prefix = s.opts.path + "/" + strings.TrimPrefix(file[0], "/")
		} else {
			prefix = s.opts.path + "/"
		}
	}

	kvs, _, err := s.opts.client.KV().List(prefix, nil)
	if err != nil {
		return nil, err
	}

	if len(kvs) == 0 {
		return nil, nil
	}

	configs := make([]*config.Configuration, 0, len(kvs))
	for _, kv := range kvs {
		fullPath := kv.Key
		path := strings.TrimPrefix(fullPath, s.opts.path)
		file := filepath.Base(fullPath)
		ext := filepath.Ext(file)
		configs = append(configs, &config.Configuration{
			Path:     path,
			File:     file,
			Name:     strings.TrimSuffix(file, ext),
			Format:   strings.TrimPrefix(ext, "."),
			Content:  kv.Value,
			FullPath: fullPath,
		})
	}

	return configs, nil
}

// Store 保存配置项
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	var key string

	if s.opts.path != "" {
		key = s.opts.path + "/" + strings.TrimPrefix(file, "/")
	} else {
		key = strings.TrimPrefix(file, "/")
	}

	_, err := s.opts.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: content,
	}, nil)

	return err
}

// Watch 监听配置项
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	return newWatcher()
}

// Close 关闭配置源
func (s *Source) Close() error {
	return nil
}
