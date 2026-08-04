/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/13 12:32 上午
 * @Desc: TODO
 */

package etcd

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const name = "etcd"

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
		r.builtin = true
		o.client, r.err = clientv3.New(clientv3.Config{
			Endpoints:   o.addrs,
			DialTimeout: o.dialTimeout,
			Username:    o.username,
			Password:    o.password,
		})
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

// 构建服务监听管理器
func (r *Registry) doBuildWatcherMgr(ctx context.Context, serviceName string) (*watcherMgr, error) {
	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr), nil
	}

	res, err := r.opts.client.Get(ctx, buildPrefixKey(r.opts.namespace, serviceName), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	r.mu1.Lock()
	defer r.mu1.Unlock()

	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr), nil
	}

	mgr := newWatcherMgr(r, serviceName, res)

	r.watchers.Store(serviceName, mgr)

	return mgr, nil
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

	return r.services(ctx, serviceName)
}

// Close 关闭服务注册发现
func (r *Registry) Close() error {
	if r.err != nil {
		return r.err
	}

	r.registrars.Range(func(key, value any) bool {
		value.(*registrar).stop()
		return true
	})

	r.watchers.Range(func(key, value any) bool {
		value.(*watcherMgr).stop()
		return true
	})

	if r.builtin {
		return r.opts.client.Close()
	}

	return nil
}

// 获取服务实例列表
func (r *Registry) services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	res, err := r.opts.client.Get(ctx, buildPrefixKey(r.opts.namespace, serviceName), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	services := make([]*registry.ServiceInstance, 0, len(res.Kvs))
	for _, kv := range res.Kvs {
		service, err := unmarshal(kv.Value)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}
