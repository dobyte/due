package consul

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/hashicorp/consul/api"
)

// Name 配置源名称
const Name = "consul"

// Source 配置源
type Source struct {
	err  error    // 构建客户端错误信息
	opts *options // 配置项
}

// NewSource 创建配置源
// 根据选项构建Consul配置中心客户端；未指定外部客户端时创建内建客户端
// @param opts ...Option 配置选项
// @return @1 config.Source 配置源
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

		s.opts.client, s.err = api.NewClient(c)
	}

	return s
}

// Name 获取配置源名称
// @return @1 string 配置源名称
func (s *Source) Name() string {
	return Name
}

// Load 加载配置项
// 传入file参数时仅加载指定的配置项；未传入file参数时，加载基础路径下所有配置项
// @param ctx context.Context 上下文
// @param file ...string 待加载的配置文件名称
// @return @1 []*config.Configuration 配置项列表
// @return @2 error 错误信息
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

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
// 仅支持write-only和read-write模式，其他模式返回无操作权限错误
// @param ctx context.Context 上下文
// @param file string 配置文件名称
// @param content []byte 配置内容
// @return @1 error 错误信息
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.err != nil {
		return s.err
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

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
// 创建监听器并监听基础路径下的配置变更
// @param ctx context.Context 上下文
// @return @1 config.Watcher 监听器
// @return @2 error 错误信息
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	return newWatcher(ctx, s)
}

// Close 关闭配置源
// @return @1 error 错误信息
func (s *Source) Close() error {
	return nil
}
