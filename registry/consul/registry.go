package consul

import (
	"context"
	"sync"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/hashicorp/consul/api"
)

const name = "consul"

var _ registry.Registry = &Registry{}

type Registry struct {
	err        error
	opts       *options
	builtin    bool
	mu1        sync.Mutex
	watchers   sync.Map
	mu2        sync.Mutex
	registrars sync.Map
}

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

		r.builtin = true
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

	reg := newRegistrar(r)

	r.registrars.Store(insID, reg)

	return reg
}

// Deregister 解注册服务实例
func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if v, ok := r.registrars.LoadAndDelete(makeInsID(ins)); ok {
		return v.(*registrar).deregister(ctx, ins)
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
	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr), nil
	}

	services, index, err := r.services(ctx, serviceName, 0, true)
	if err != nil {
		return nil, err
	}

	r.mu1.Lock()
	defer r.mu1.Unlock()

	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr), nil
	}

	mgr := newWatcherMgr(r, serviceName, services, index)

	r.watchers.Store(serviceName, mgr)

	return mgr, nil
}

// Close 关闭服务注册发现
func (r *Registry) Close() error {
	if r.err != nil {
		return r.err
	}

	r.registrars.Range(func(key, value any) bool {
		value.(*registrar).stop()
		r.registrars.Delete(key)
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
		if services, err := v.(*watcherMgr).services(); err == nil {
			return services, nil
		}
	}

	return r.queryServices(ctx, serviceName)
}

func (r *Registry) queryServices(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	services, _, err := r.services(ctx, serviceName, 0, true)
	return services, err
}

func (r *Registry) services(ctx context.Context, serviceName string, waitIndex uint64, passingOnly bool) ([]*registry.ServiceInstance, uint64, error) {
	opts := &api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  60 * time.Second,
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
			Routes:   unmarshalMetaRoutes(entry.Service.Meta),
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
			case metaFieldEvents:
				if err = json.Unmarshal(xconv.StringToBytes(v), &ins.Events); err != nil {
					continue
				}
			case metaFieldServices:
				if err = json.Unmarshal(xconv.StringToBytes(v), &ins.Services); err != nil {
					continue
				}
			case metaFieldEndpoint:
				ins.Endpoint = v
			default:
				if len(k) > 0 && string(k[0]) == defaultMetadataPrefix {
					ins.Metadata[string(k[1:])] = v
				}
			}
		}

		services = append(services, ins)
	}

	return services, meta.LastIndex, nil
}
