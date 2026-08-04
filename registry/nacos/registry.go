package nacos

import (
	"context"
	stdnet "net"
	"net/url"
	"sync"

	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

const name = "nacos"

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
	mu       sync.Mutex
	watchers sync.Map
}

func NewRegistry(opts ...Option) *Registry {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Registry{}
	r.opts = o

	if o.client == nil {
		serverConfigs := make([]constant.ServerConfig, 0, len(o.urls))

		for _, v := range o.urls {
			raw, err := url.Parse(v)
			if err != nil {
				log.Warnf("%s parse failed: %v", v, err)
				continue
			}

			host, port, err := stdnet.SplitHostPort(raw.Host)
			if err != nil {
				log.Warnf("%s parse failed: %v", v, err)
				continue
			}

			serverConfigs = append(serverConfigs, constant.ServerConfig{
				Scheme:      raw.Scheme,
				ContextPath: raw.Path,
				IpAddr:      host,
				Port:        xconv.Uint64(port),
			})
		}

		if len(serverConfigs) == 0 {
			r.err = errors.ErrInvalidArgument
		} else {
			r.builtin = true
			o.client, r.err = clients.NewNamingClient(vo.NacosClientParam{
				ServerConfigs: serverConfigs,
				ClientConfig: &constant.ClientConfig{
					TimeoutMs:            uint64(r.opts.timeout.Milliseconds()),
					BeatInterval:         int64(r.opts.heartbeat.Milliseconds()),
					NamespaceId:          r.opts.namespaceId,
					Endpoint:             r.opts.endpoint,
					RegionId:             r.opts.regionId,
					AccessKey:            r.opts.accessKey,
					SecretKey:            r.opts.secretKey,
					OpenKMS:              r.opts.openKMS,
					CacheDir:             r.opts.cacheDir,
					Username:             r.opts.username,
					Password:             r.opts.password,
					LogDir:               r.opts.logDir,
					LogLevel:             r.opts.logLevel,
					NotLoadCacheAtStart:  true,
					UpdateCacheWhenEmpty: true,
					UpdateThreadNum:      20,
				},
			})
		}
	}

	return r
}

// Name 服务注册发现组件名
func (r *Registry) Name() string {
	return name
}

// Register 注册服务实例
func (r *Registry) Register(_ context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	host, port, err := net.ParseHostPort(ins.Endpoint)
	if err != nil {
		return err
	}

	param := vo.RegisterInstanceParam{
		Ip:          host,
		Port:        port,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		ServiceName: ins.Name,
		ClusterName: r.opts.clusterName,
		GroupName:   r.opts.groupName,
		Metadata:    make(map[string]string, 11),
	}

	param.Metadata[metaFieldID] = ins.ID
	param.Metadata[metaFieldName] = ins.Name
	param.Metadata[metaFieldKind] = ins.Kind
	param.Metadata[metaFieldAlias] = ins.Alias
	param.Metadata[metaFieldState] = ins.State
	param.Metadata[metaFieldEndpoint] = ins.Endpoint

	if ins.Weight > 0 {
		param.Weight = float64(ins.Weight)
		param.Metadata[metaFieldWeight] = xconv.String(ins.Weight)
	} else {
		param.Weight = 1
	}

	if len(ins.Routes) > 0 {
		if routes, err := json.Marshal(ins.Routes); err != nil {
			return err
		} else {
			param.Metadata[metaFieldRoutes] = xconv.BytesToString(routes)
		}
	}

	if len(ins.Events) > 0 {
		if events, err := json.Marshal(ins.Events); err != nil {
			return err
		} else {
			param.Metadata[metaFieldEvents] = xconv.BytesToString(events)
		}
	}

	if len(ins.Services) > 0 {
		if services, err := json.Marshal(ins.Services); err != nil {
			return err
		} else {
			param.Metadata[metaFieldServices] = xconv.BytesToString(services)
		}
	}

	if len(ins.Metadata) > 0 {
		if metadata, err := json.Marshal(ins.Metadata); err != nil {
			return err
		} else {
			param.Metadata[metaFieldMetadata] = xconv.BytesToString(metadata)
		}
	}

	if ok, err := r.opts.client.RegisterInstance(param); err != nil {
		return err
	} else if !ok {
		return errors.ErrServiceRegisterFailed
	}

	return nil
}

// Deregister 解注册服务实例
func (r *Registry) Deregister(_ context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	host, port, err := net.ParseHostPort(ins.Endpoint)
	if err != nil {
		return err
	}

	ok, err := r.opts.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: ins.Name,
		Cluster:     r.opts.clusterName,
		GroupName:   r.opts.groupName,
		Ephemeral:   true,
	})
	if err != nil {
		return err
	}

	if !ok {
		return errors.ErrServiceDeregisterFailed
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

// 构建服务实例监听器
func (r *Registry) doBuildWatcherMgr(ctx context.Context, serviceName string) (*watcherMgr, error) {
	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr), nil
	}

	services, err := r.services(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr), nil
	}

	mgr, err := newWatcherMgr(r, serviceName, services)
	if err != nil {
		return nil, err
	}

	r.watchers.Store(serviceName, mgr)

	return mgr, nil
}

// Services 获取服务实例列表
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	if v, ok := r.watchers.Load(serviceName); ok {
		if services, err := v.(*watcherMgr).services(); err != nil {
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

	r.mu.Lock()
	r.watchers.Range(func(key, value any) bool {
		value.(*watcherMgr).stop()
		return true
	})
	r.mu.Unlock()

	if r.builtin {
		r.opts.client.CloseClient()
	}

	return nil
}

// 获取服务实例列表
func (r *Registry) services(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	instances, err := r.opts.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		Clusters:    []string{r.opts.clusterName},
		GroupName:   r.opts.groupName,
		HealthyOnly: true,
	})
	if err != nil && instances == nil {
		return nil, err
	}

	return parseInstances(instances)
}
