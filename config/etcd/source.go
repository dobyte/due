package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Name 配置源名称
const Name = "etcd"

// Source 配置源
type Source struct {
	err     error    // 构建客户端错误信息
	opts    *options // 配置项
	builtin bool     // 是否为内建客户端
}

// NewSource 创建配置源
// 根据选项构建etcd配置中心客户端；未指定外部客户端时创建内建客户端
// @param opts ...Option 配置选项
// @return @1 config.Source 配置源
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o

	// 归一化路径并强制追加尾部斜杠，避免WithPrefix误匹配兄弟命名空间的键（如/config2、/confighost）
	path := strings.Trim(o.path, "/")
	if path == "" {
		log.Warnf("invalid config path, use default path: %s", defaultPath)
		path = strings.Trim(defaultPath, "/")
	}
	s.opts.path = fmt.Sprintf("/%s/", path)

	if o.client == nil {
		s.builtin = true
		o.client, s.err = clientv3.New(clientv3.Config{
			Endpoints:   o.addrs,
			DialTimeout: o.dialTimeout,
			Username:    o.username,
			Password:    o.password,
		})
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

	var (
		key  = s.opts.path
		opts []clientv3.OpOption
	)

	if len(file) > 0 && file[0] != "" {
		key += strings.TrimPrefix(file[0], "/")
	} else {
		opts = append(opts, clientv3.WithPrefix())
	}

	res, err := s.opts.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	configs := make([]*config.Configuration, 0, len(res.Kvs))
	for _, kv := range res.Kvs {
		fullPath := string(kv.Key)
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

	key := s.opts.path + strings.TrimPrefix(file, "/")
	_, err := s.opts.client.Put(ctx, key, xconv.String(content))
	return err
}

// Watch 监听配置项
// 先全量拉取一次配置作为初始快照，再创建监听器监听后续变更
// @param ctx context.Context 上下文
// @return @1 config.Watcher 监听器
// @return @2 error 错误信息
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	// 先全量拉取一次配置，作为监听初始快照，并记录监听起始版本号
	res, err := s.opts.client.Get(ctx, s.opts.path, clientv3.WithPrefix())
	if err != nil {
		log.Warnf("etcd watch get failed: %v", err)
		res = nil
	}

	return newWatcher(ctx, s, res)
}

// Close 关闭资源
// 内建客户端时关闭客户端连接，外部客户端由调用方负责关闭
// @return @1 error 错误信息
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	if s.builtin {
		return s.opts.client.Close()
	}

	return nil
}
