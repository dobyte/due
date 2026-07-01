package nacos

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"sync"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

const name = "nacos"

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
				if e != nil {
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
				r.err = errors.ErrInvalidArgument
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

func (r *Registry) Name() string {
	return name
}

func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	return r.doBuildRegistrar(makeInsID(ins)).register(ctx, ins)
}

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

func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if v, ok := r.registrars.LoadAndDelete(makeInsID(ins)); ok {
		return v.(*registrar).deregister(ctx, ins)
	}

	return nil
}

func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.err != nil {
		return nil, r.err
	}

	mgr, err := r.doBuildWatcherMgr(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	if w := mgr.fork(); w != nil {
		return w, nil
	}

	return nil, errors.ErrWatcherStopped
}

func (r *Registry) doBuildWatcherMgr(ctx context.Context, serviceName string) (*watcherMgr, error) {
	if v, ok := r.watchers.Load(serviceName); ok {
		return v.(*watcherMgr), nil
	}

	services, err := r.services(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	r.mu1.Lock()
	defer r.mu1.Unlock()

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

func (r *Registry) services(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
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
		ins.Services = make([]string, 0)
		ins.Weight = xconv.Int(instance.Metadata[metaFieldWeight])
		ins.Metadata = make(map[string]string)

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

		if v := instance.Metadata[metaFieldServices]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Services); err != nil {
				return nil, err
			}
		}

		if v := instance.Metadata[metaFieldMetadata]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Metadata); err != nil {
				return nil, err
			}
		}

		services = append(services, ins)
	}

	return services, nil
}
