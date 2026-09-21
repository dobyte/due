package consul

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/hashicorp/consul/api"
)

// Name 配置源名称
const Name = "consul"

// Source 配置源
type Source struct {
	err       error           // 构建客户端错误信息
	opts      *options        // 配置项
	builtin   bool            // 是否为内建客户端
	transport *http.Transport // 内建客户端的底层传输层
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

	// 归一化路径，去除首尾斜杠；路径为空时告警并回退默认值，
	// 避免空路径导致List/监听覆盖Consul全量键
	path := strings.Trim(s.opts.path, "/")
	if path == "" {
		log.Warnf("invalid config path, use default path: %s", defaultPath)
		path = strings.Trim(defaultPath, "/")
	}
	s.opts.path = path

	if o.client == nil {
		c := api.DefaultConfig()
		if o.addr != "" {
			c.Address = o.addr
		}

		s.builtin = true
		s.transport = c.Transport
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

	// 传入file参数时按精确键查询，仅返回目标配置项
	if len(file) > 0 && file[0] != "" {
		key := s.opts.path + "/" + strings.TrimPrefix(file[0], "/")

		kv, _, err := s.opts.client.KV().Get(key, (&api.QueryOptions{}).WithContext(ctx))
		if err != nil {
			return nil, err
		}

		if kv == nil {
			return nil, nil
		}

		return []*config.Configuration{s.parseKV(kv.Key, kv.Value)}, nil
	}

	// 未传入file参数时，加载基础路径下所有配置项
	kvs, _, err := s.opts.client.KV().List(s.opts.path+"/", (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return nil, err
	}

	configs := make([]*config.Configuration, 0, len(kvs))
	for _, kv := range kvs {
		configs = append(configs, s.parseKV(kv.Key, kv.Value))
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

	key := s.opts.path + "/" + strings.TrimPrefix(file, "/")

	_, err := s.opts.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: content,
	}, (&api.WriteOptions{}).WithContext(ctx))

	return err
}

// parseKV 解析Consul的键值对为统一的配置结构
// @param key string 配置键名
// @param value []byte 配置内容
// @return @1 *config.Configuration 配置项
func (s *Source) parseKV(key string, value []byte) *config.Configuration {
	fullPath := key
	relPath := strings.TrimPrefix(fullPath, s.opts.path)
	relPath = strings.TrimPrefix(relPath, "/")
	file := path.Base(fullPath)
	ext := path.Ext(file)

	return &config.Configuration{
		Path:     relPath,
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  value,
		FullPath: fullPath,
	}
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
// 内建客户端时关闭其底层传输层的空闲连接，外部客户端由调用方负责关闭
// @return @1 error 错误信息
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	if s.builtin && s.transport != nil {
		s.transport.CloseIdleConnections()
	}

	return nil
}
