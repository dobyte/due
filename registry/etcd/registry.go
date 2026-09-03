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

// Registry 服务注册发现组件
// 基于 etcd 实现服务注册与发现，支持注册/解注册、监听服务实例变化及查询服务实例
type Registry struct {
	err        error              // 初始化错误（内建客户端创建失败时记录）
	opts       *options           // 配置项
	builtin    bool               // 是否使用内建客户端
	ctx        context.Context    // 组件根上下文，保活/监听协程均派生自它，确保随组件销毁联动终止
	cancel     context.CancelFunc // 组件根上下文取消函数
	mu1        sync.Mutex         // 保护 watchers 注册表
	watchers   sync.Map           // 服务监听管理器注册表
	mu2        sync.Mutex         // 保护 registrars 注册表
	registrars sync.Map           // 服务注册器注册表
}

// NewRegistry 创建服务注册发现组件
// @param opts ...Option 服务注册发现配置项
// @return @1 *Registry 服务注册发现组件实例
func NewRegistry(opts ...Option) *Registry {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Registry{}
	r.opts = o
	r.ctx, r.cancel = context.WithCancel(context.Background())

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
// 同一实例ID对应的注册器在注册表中复用，避免重复创建导致保活流错乱
// @param insID string 服务实例ID
// @return @1 *registrar 服务注册器实例
func (r *Registry) doBuildRegistrar(insID string) *registrar {
	if v, ok := r.registrars.Load(insID); ok {
		return v.(*registrar)
	}

	r.mu2.Lock()
	defer r.mu2.Unlock()

	if v, ok := r.registrars.Load(insID); ok {
		return v.(*registrar)
	} else {
		return newRegistrar(r, insID)
	}
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
// 同一服务名的监听管理器在注册表中复用；仅当注册表中不存在或已停止时才重建
// @param ctx context.Context 上下文
// @param serviceName string 服务名称
// @return @1 *watcherMgr 服务监听管理器实例
// @return @2 error 构建失败时返回的错误
func (r *Registry) doBuildWatcherMgr(ctx context.Context, serviceName string) (*watcherMgr, error) {
	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		return mgr, nil
	}

	res, err := r.opts.client.Get(ctx, buildPrefixKey(r.opts.namespace, serviceName), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	r.mu1.Lock()
	defer r.mu1.Unlock()

	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		return mgr, nil
	} else {
		mgr := newWatcherMgr(r, serviceName, res)
		r.watchers.Store(serviceName, mgr)
		mgr.init()

		return mgr, nil
	}
}

// loadWatcherMgr 加载服务监听管理器
// @param serviceName string 服务名称
// @return @1 *watcherMgr 服务监听管理器实例
func (r *Registry) loadWatcherMgr(serviceName string) *watcherMgr {
	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && !mgr.stopped.Load() {
			return mgr
		}
	}

	return nil
}

// Services 获取服务实例列表
// watch 链路健康时返回缓存数据，否则回源 etcd 实时查询，避免提供过期数据
// @param ctx context.Context 上下文
// @param serviceName string 服务名称
// @return @1 []*registry.ServiceInstance 服务实例列表
// @return @2 error 获取失败时返回的错误
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr := v.(*watcherMgr); mgr.health.Load() {
			if services, err := mgr.services(); err == nil {
				return services, nil
			}
		}
	}

	return r.services(ctx, serviceName)
}

// Close 关闭服务注册发现
// 停止全部服务注册器与监听管理器并释放资源；使用内建客户端时一并关闭客户端
// @return @1 error 关闭失败时返回的错误
func (r *Registry) Close() error {
	if r.err != nil {
		return r.err
	}

	// 先取消组件根上下文，联动终止所有派生的保活/监听协程
	r.cancel()

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
