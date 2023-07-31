package etcd

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/config/configurator"
	"github.com/dobyte/due/v2/utils/xconv"
	"go.etcd.io/etcd/client/v3"
	"path/filepath"
	"strings"
)

const Name = "etcd"

type Source struct {
	err     error
	opts    *options
	builtin bool
}

func NewSource(opts ...Option) *Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o
	s.opts.path = fmt.Sprintf("/%s", strings.TrimSuffix(strings.TrimPrefix(s.opts.path, "/"), "/"))

	if o.client == nil {
		s.builtin = true
		o.client, s.err = clientv3.New(clientv3.Config{
			Endpoints:   o.addrs,
			DialTimeout: o.dialTimeout,
		})
	}

	return s
}

// Name 配置源名称
func (s *Source) Name() string {
	return Name
}

// Load 加载配置项
func (s *Source) Load(ctx context.Context, file ...string) ([]*configurator.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	var (
		key  = s.opts.path
		opts []clientv3.OpOption
	)

	if len(file) > 0 && file[0] != "" {
		key += "/" + strings.TrimPrefix(file[0], "/")
	} else {
		opts = append(opts, clientv3.WithPrefix())
	}

	res, err := s.opts.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	configs := make([]*configurator.Configuration, 0, len(res.Kvs))
	for _, kv := range res.Kvs {
		fullPath := string(kv.Key)
		path := strings.TrimPrefix(fullPath, s.opts.path)
		file := filepath.Base(fullPath)
		ext := filepath.Ext(file)
		configs = append(configs, &configurator.Configuration{
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
	if s.err != nil {
		return s.err
	}

	if s.opts.mode != "read-write" {
		return configurator.ErrNoOperationPermission
	}

	key := s.opts.path + "/" + strings.TrimPrefix(file, "/")
	_, err := s.opts.client.Put(ctx, key, xconv.String(content))
	return err
}

// Watch 监听配置项
func (s *Source) Watch(ctx context.Context) (configurator.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	return newWatcher(ctx, s)
}
