// Package consul 提供基于 Consul 的服务注册发现组件，实现了 registry.Registry 接口，
// 支持服务实例的注册、解注册、监听与查询，并支持健康检查与心跳保活。
package consul

import (
	"context"
	"sync"
	"time"

	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/hashicorp/consul/api"
)

// name 是服务注册发现组件的名称
const name = "consul"

var _ registry.Registry = &Registry{}

// Registry 是基于 Consul 的服务注册发现组件，实现了 registry.Registry 接口。
type Registry struct {
	err        error      // 初始化错误（内建客户端创建失败时记录）
	opts       *options   // 配置项
	mu1        sync.Mutex // 保护 watchers 注册表
	watchers   sync.Map   // 服务监听管理器注册表
	mu2        sync.Mutex // 保护 registrars 注册表
	registrars sync.Map   // 服务注册器注册表
}

// NewRegistry 创建基于 Consul 的服务注册发现组件实例。
func NewRegistry(opts ...Option) *Registry {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Registry{}
	r.opts = o

	if o.client == nil {
		config := api.DefaultConfig()
		if o.addr != "" {
			config.Address = o.addr
		}

		o.client, r.err = api.NewClient(config)
	}

	return r
}

// Name 获取服务注册发现组件名
func (r *Registry) Name() string {
	return name
}

// Register 注册服务实例
func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	return r.doBuildRegistrar(makeInsID(ins)).register(ctx, ins)
}

// 构建服务注册器
func (r *Registry) doBuildRegistrar(insID string) *registrar {
	if v, ok := r.registrars.Load(insID); ok {
		return v.(*registrar)
	}

	r.mu2.Lock()
	defer r.mu2.Unlock()

	if v, ok := r.registrars.Load(insID); ok {
		return v.(*registrar)
	}

	reg := newRegistrar(r, insID)

	r.registrars.Store(insID, reg)

	return reg
}

// Deregister 解注册服务实例
func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if v, ok := r.registrars.Load(makeInsID(ins)); ok {
		return v.(*registrar).deregister(ctx)
	}

	return nil
}

// Watch 监听相同服务名的服务实例变化
func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.err != nil {
		return nil, r.err
	}

	mgr, err := r.doBuildWatcherMgr(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	return mgr.fork()
}

// 构建服务监听器管理器
func (r *Registry) doBuildWatcherMgr(ctx context.Context, serviceName string) (*watcherMgr, error) {
	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		return mgr, nil
	}

	services, index, err := r.services(ctx, serviceName, 0, true, false)
	if err != nil {
		return nil, err
	}

	r.mu1.Lock()
	defer r.mu1.Unlock()

	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		return mgr, nil
	} else {
		mgr := newWatcherMgr(r, serviceName, services, index)
		r.watchers.Store(serviceName, mgr)
		mgr.init()

		return mgr, nil
	}
}

// loadWatcherMgr 加载服务监听管理器，不存在或已停止时返回 nil。
func (r *Registry) loadWatcherMgr(serviceName string) *watcherMgr {
	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && !mgr.stopped.Load() {
			return mgr
		}
	}

	return nil
}

// Close 关闭服务注册发现
func (r *Registry) Close() error {
	if r.err != nil {
		return r.err
	}

	r.registrars.Range(func(key, value any) bool {
		value.(*registrar).close()
		return true
	})

	r.watchers.Range(func(key, value any) bool {
		value.(*watcherMgr).stop()
		return true
	})

	return nil
}

// Services 获取服务实例列表
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	if v, ok := r.watchers.Load(serviceName); ok {
		mgr := v.(*watcherMgr)
		if mgr.healthy.Load() {
			if services, err := mgr.services(); err == nil {
				return services, nil
			}
		}
	}

	services, _, err := r.services(ctx, serviceName, 0, true, false)
	return services, err
}

// services 从 Consul 查询指定服务的健康实例列表，并返回最新索引。
func (r *Registry) services(ctx context.Context, serviceName string, waitIndex uint64, passingOnly, blocking bool) ([]*registry.ServiceInstance, uint64, error) {
	opts := &api.QueryOptions{
		WaitIndex: waitIndex,
	}

	// 阻塞查询需要设置 WaitTime，非阻塞查询会立即返回
	if blocking {
		opts.WaitTime = 60 * time.Second
	}

	opts = opts.WithContext(ctx)

	entries, meta, err := r.opts.client.Health().Service(serviceName, "", passingOnly, opts)
	if err != nil {
		return nil, 0, err
	}

	services := make([]*registry.ServiceInstance, 0, len(entries))
	for _, entry := range entries {
		ins := &registry.ServiceInstance{
			Name:     entry.Service.Service,
			Events:   make([]int, 0),
			Services: make([]string, 0),
			Metadata: make(map[string]string),
		}

		for k, v := range entry.Service.Meta {
			switch k {
			case metaFieldID:
				ins.ID = v
			case metaFieldKind:
				ins.Kind = v
			case metaFieldAlias:
				ins.Alias = v
			case metaFieldState:
				ins.State = v
			case metaFieldWeight:
				ins.Weight = xconv.Int(v)
			case metaFieldEndpoint:
				ins.Endpoint = v
			default:
				if len(k) > 0 && k[0] == defaultMetadataPrefix[0] {
					ins.Metadata[k[1:]] = v
				}
			}
		}

		// 跳过非 due 框架注册的服务实例（缺少 ID 元数据），避免混入非法实例
		if ins.ID == "" {
			continue
		}

		ins.Routes = unmarshalMetaRoutes(entry.Service.Meta)

		if events, err := unmarshalMetaList[int](metaFieldEvents, entry.Service.Meta); err != nil {
			log.Warnf("consul unmarshal meta events failed: %v", err)
		} else {
			ins.Events = events
		}

		if subs, err := unmarshalMetaList[string](metaFieldServices, entry.Service.Meta); err != nil {
			log.Warnf("consul unmarshal meta services failed: %v", err)
		} else {
			ins.Services = subs
		}

		services = append(services, ins)
	}

	return services, meta.LastIndex, nil
}
