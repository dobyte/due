package nacos

import (
	"context"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"net"
	"net/url"
	"strconv"
	"sync"
)

const name = "nacos"

var _ registry.Registry = &Registry{}

type Registry struct {
	err        error
	ctx        context.Context
	cancel     context.CancelFunc
	opts       *options
	builtin    bool
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
		param := vo.NacosClientParam{
			ServerConfigs: make([]constant.ServerConfig, 0, len(o.urls)),
			ClientConfig: &constant.ClientConfig{
				TimeoutMs:            uint64(r.opts.timeout.Microseconds()),
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
			},
		}

		var (
			err      error
			endpoint string
		)

		for _, v := range o.urls {
			if raw, e := url.Parse(v); e != nil {
				err, endpoint = e, v
			} else {
				host, p, e := net.SplitHostPort(raw.Host)
				if err != nil {
					err, endpoint = e, v
					continue
				}

				port, e := strconv.ParseUint(p, 10, 64)
				if e != nil {
					err, endpoint = e, v
					continue
				}

				param.ServerConfigs = append(param.ServerConfigs, constant.ServerConfig{
					Scheme:      raw.Scheme,
					ContextPath: raw.Path,
					IpAddr:      host,
					Port:        port,
				})
			}
		}

		if len(param.ServerConfigs) == 0 {
			if err != nil {
				r.err = err
			} else {
				r.err = errors.New("invalid server urls")
			}
		} else {
			if err != nil {
				log.Warnf("%s parse failed: %v", endpoint, err)
			}

			o.client, r.err = clients.NewNamingClient(param)
			r.builtin = true
		}
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
	if r.err != nil {
		return r.err
	}

	if v, ok := r.registrars.Load(ins.ID); ok {
		return v.(*registrar).deregister(ctx, ins)
	}

	return nil
}

// Watch 监听相同服务名的服务实例变化
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

// Services 获取服务实例列表
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr).services(), nil
	} else {
		return r.services(ctx, serviceName)
	}
}

// 获取服务实体列表
func (r *Registry) services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	instances, err := r.opts.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		Clusters:    []string{r.opts.clusterName},
		GroupName:   r.opts.groupName,
		HealthyOnly: true,
	})
	if err != nil {
		if instances == nil {
			return nil, err
		} else {
			return nil, nil
		}
	}

	return parseInstances(instances)
}

func parseInstances(instances []model.Instance) ([]*registry.ServiceInstance, error) {
	services := make([]*registry.ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		if !instance.Healthy || !instance.Enable {
			continue
		}

		ins := &registry.ServiceInstance{}
		ins.ID = instance.Metadata[metaFieldID]
		ins.Name = instance.Metadata[metaFieldName]
		ins.Kind = instance.Metadata[metaFieldKind]
		ins.Alias = instance.Metadata[metaFieldAlias]
		ins.State = instance.Metadata[metaFieldState]
		ins.Endpoint = instance.Metadata[metaFieldEndpoint]
		ins.Routes = make([]registry.Route, 0)
		ins.Events = make([]int, 0)

		if v := instance.Metadata[metaFieldRoutes]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Routes); err != nil {
				return nil, err
			}
		}

		if v := instance.Metadata[metaFieldEvents]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Events); err != nil {
				return nil, err
			}
		}

		services = append(services, ins)
	}

	return services, nil
}
