package polaris

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/polarismesh/polaris-go/api"
	"golang.org/x/sync/singleflight"
)

// name 注册中心组件名称
const name = "polaris"

// maxWeight 服务实例权重上限
// polaris 服务实例权重的合法范围为 [0, 10000]
const maxWeight = 10000

const (
	metaFieldID       = "id"
	metaFieldName     = "name"
	metaFieldKind     = "kind"
	metaFieldAlias    = "alias"
	metaFieldState    = "state"
	metaFieldRoutes   = "routes"
	metaFieldEvents   = "events"
	metaFieldWeight   = "weight"
	metaFieldServices = "services"
	metaFieldEndpoint = "endpoint"
	metaFieldMetadata = "metadata"
)

var _ registry.Registry = &Registry{}

type Registry struct {
	err      error
	opts     *options
	builtin  bool
	provider api.ProviderAPI
	consumer api.ConsumerAPI
	watchers sync.Map
	group    singleflight.Group
	closed   atomic.Bool
}

func NewRegistry(opts ...Option) *Registry {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Registry{}
	r.opts = o

	if o.client == nil {
		o.client, r.err = r.buildClient()
		r.builtin = true
	}

	if r.err == nil {
		r.provider = api.NewProviderAPIByContext(o.client)
		r.consumer = api.NewConsumerAPIByContext(o.client)
	}

	return r
}

// Name 服务注册发现组件名
func (r *Registry) Name() string {
	return name
}

// Register 注册服务实例
func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if r.closed.Load() {
		return errors.ErrRegistryClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	host, port, protocol, err := parseEndpoint(ins.Endpoint)
	if err != nil {
		return err
	}

	weight := min(max(ins.Weight, 1), maxWeight)

	req := &api.InstanceRegisterRequest{}
	req.Service = ins.Name
	req.Namespace = r.opts.namespace
	req.Host = host
	req.Port = port
	req.Protocol = &protocol
	req.Weight = &weight
	req.Metadata = make(map[string]string, 11)

	req.Metadata[metaFieldID] = ins.ID
	req.Metadata[metaFieldName] = ins.Name
	req.Metadata[metaFieldKind] = ins.Kind
	req.Metadata[metaFieldAlias] = ins.Alias
	req.Metadata[metaFieldState] = ins.State
	req.Metadata[metaFieldEndpoint] = ins.Endpoint
	req.Metadata[metaFieldWeight] = xconv.String(weight)

	if len(ins.Routes) > 0 {
		if routes, err := json.Marshal(ins.Routes); err != nil {
			return err
		} else {
			req.Metadata[metaFieldRoutes] = xconv.BytesToString(routes)
		}
	}

	if len(ins.Events) > 0 {
		if events, err := json.Marshal(ins.Events); err != nil {
			return err
		} else {
			req.Metadata[metaFieldEvents] = xconv.BytesToString(events)
		}
	}

	if len(ins.Services) > 0 {
		if services, err := json.Marshal(ins.Services); err != nil {
			return err
		} else {
			req.Metadata[metaFieldServices] = xconv.BytesToString(services)
		}
	}

	if len(ins.Metadata) > 0 {
		if metadata, err := json.Marshal(ins.Metadata); err != nil {
			return err
		} else {
			req.Metadata[metaFieldMetadata] = xconv.BytesToString(metadata)
		}
	}

	resp, err := r.provider.RegisterInstance(req)
	if err != nil {
		return err
	}

	// Existed 表示实例在注册前是否已存在；由于注册是幂等的，
	// 重复注册（如更新实例状态）时 Existed 为 true，仍视为注册成功
	if resp.Existed {
		log.Debugf("instance %s re-registered", ins.ID)
	}

	return nil
}

// Deregister 解注册服务实例
func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if r.closed.Load() {
		return errors.ErrRegistryClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	host, port, _, err := parseEndpoint(ins.Endpoint)
	if err != nil {
		return err
	}

	req := &api.InstanceDeRegisterRequest{}
	req.Service = ins.Name
	req.Namespace = r.opts.namespace
	req.Host = host
	req.Port = port

	if err := r.provider.Deregister(req); err != nil {
		return err
	}

	return nil
}

// Watch 监听相同服务名的服务实例变化
func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.err != nil {
		return nil, r.err
	}

	if r.closed.Load() {
		return nil, errors.ErrRegistryClosed
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mgr, err := r.doBuildWatcherMgr(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	return mgr.fork()
}

// 构建服务实例监听器
func (r *Registry) doBuildWatcherMgr(_ context.Context, serviceName string) (*watcherMgr, error) {
	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		return mgr, nil
	}

	v, err, _ := r.group.Do(serviceName, func() (any, error) {
		if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
			return mgr, nil
		}

		mgr := newWatcherMgr(r, serviceName)
		if err := mgr.init(); err != nil {
			return nil, err
		}

		r.watchers.Store(serviceName, mgr)

		// 防止 Close 与 Watch 并发时，Store 进一个已经关闭的监听管理器
		if r.closed.Load() {
			mgr.stop()
			return nil, errors.ErrRegistryClosed
		}

		return mgr, nil
	})
	if err != nil {
		return nil, err
	}

	return v.(*watcherMgr), nil
}

// 加载服务监听管理器
// 仅返回未停止的管理器，避免返回已停止但尚未从注册表移除的管理器
func (r *Registry) loadWatcherMgr(serviceName string) *watcherMgr {
	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && !mgr.stopped.Load() {
			return mgr
		}
	}

	return nil
}

// Services 获取服务实例列表
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	if r.closed.Load() {
		return nil, errors.ErrRegistryClosed
	}

	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		if services, err := mgr.services(); err != nil {
			log.Warnf("load %s services failed: %v", serviceName, err)
		} else {
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

	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}

	r.watchers.Range(func(key, value any) bool {
		value.(*watcherMgr).stop()
		return true
	})

	if r.builtin {
		r.opts.client.Destroy()
	}

	return nil
}

// services 获取服务实例列表
func (r *Registry) services(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	req := &api.GetAllInstancesRequest{}
	req.Service = serviceName
	req.Namespace = r.opts.namespace

	resp, err := r.consumer.GetAllInstances(req)
	if err != nil {
		return nil, err
	}

	return parseInstances(resp.GetInstances())
}

// buildClient 构建Polaris SDK上下文
func (r *Registry) buildClient() (api.SDKContext, error) {
	cfg := api.NewConfiguration()

	cfg.GetGlobal().GetServerConnector().SetAddresses(r.opts.urls)
	cfg.GetGlobal().GetServerConnector().SetProtocol(r.opts.protocol)
	cfg.GetGlobal().GetServerConnector().SetMessageTimeout(r.opts.timeout)
	cfg.GetGlobal().GetAPI().SetTimeout(r.opts.timeout)

	return api.InitContextByConfig(cfg)
}
