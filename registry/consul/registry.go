package consul

import (
	"context"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/hashicorp/consul/api"
	"sync"
	"time"
)

const name = "consul"

var _ registry.Registry = &Registry{}

type Registry struct {
	err        error
	ctx        context.Context
	cancel     context.CancelFunc
	opts       *options
	watchers   sync.Map
	registrars sync.Map
}

func NewRegistry(opts ...Option) *Registry {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Registry{}
	r.opts = o
	r.ctx, r.cancel = context.WithCancel(o.ctx)

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

	v, ok := r.registrars.Load(ins.ID)
	if ok {
		return v.(*registrar).register(ctx, ins)
	}

	reg := newRegistrar(r)

	if err := reg.register(ctx, ins); err != nil {
		return err
	}

	r.registrars.Store(ins.ID, reg)

	return nil
}

// Deregister 解注册服务实例
func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	v, ok := r.registrars.Load(ins.ID)
	if ok {
		return v.(*registrar).deregister(ctx, ins)
	}

	return r.opts.client.Agent().ServiceDeregister(ins.ID)
}

// Services 获取服务实例列表
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	v, ok := r.watchers.Load(serviceName)
	if ok {
		return v.(*watcherMgr).services(), nil
	} else {
		services, _, err := r.services(ctx, serviceName, 0, true)
		return services, err
	}
}

// Watch 监听服务
func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.err != nil {
		return nil, r.err
	}

	v, ok := r.watchers.Load(serviceName)
	if ok {
		return v.(*watcherMgr).fork(), nil
	}

	w, err := newWatcherMgr(r, ctx, serviceName)
	if err != nil {
		return nil, err
	}
	r.watchers.Store(serviceName, w)

	return w.fork(), nil
}

// 获取服务实体列表
func (r *Registry) services(ctx context.Context, serviceName string, waitIndex uint64, passingOnly bool) ([]*registry.ServiceInstance, uint64, error) {
	opts := &api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  60 * time.Second,
	}
	opts.WithContext(ctx)

	entries, meta, err := r.opts.client.Health().Service(serviceName, "", passingOnly, opts)
	if err != nil {
		return nil, 0, err
	}

	services := make([]*registry.ServiceInstance, 0, len(entries))
	for _, entry := range entries {
		ins := &registry.ServiceInstance{
			ID:       entry.Service.ID,
			Name:     entry.Service.Service,
			Routes:   unmarshalMetaRoutes(entry.Service.Meta),
			Events:   make([]int, 0),
			Services: make([]string, 0),
		}

		for k, v := range entry.Service.Meta {
			switch k {
			case metaFieldKind:
				ins.Kind = v
			case metaFieldAlias:
				ins.Alias = v
			case metaFieldState:
				ins.State = v
			case metaFieldWeight:
				ins.Weight = xconv.Int(v)
			case metaFieldEvents:
				if err = json.Unmarshal([]byte(v), &ins.Events); err != nil {
					continue
				}
			case metaFieldServices:
				if err = json.Unmarshal([]byte(v), &ins.Services); err != nil {
					continue
				}
			case metaFieldEndpoint:
				ins.Endpoint = v
			}
		}

		services = append(services, ins)
	}

	return services, meta.LastIndex, nil
}
