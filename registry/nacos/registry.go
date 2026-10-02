// Package nacos provides a Nacos-backed implementation of service registry and discovery.
//
// It supports registering and deregistering service instances, watching the instance changes of a
// service, and querying service instances.
package nacos

import (
	"context"
	stdnet "net"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"golang.org/x/sync/singleflight"
)

// name is the name of the registry component.
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

// Registry is a service registry and discovery component backed by Nacos.
type Registry struct {
	err      error
	opts     *options
	builtin  bool
	watchers sync.Map
	group    singleflight.Group
	closed   atomic.Bool
}

// NewRegistry returns a new service registry and discovery component.
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

// Name returns the name of the service registry and discovery component.
func (r *Registry) Name() string {
	return name
}

// Register registers a service instance.
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
		Weight:      float64(max(ins.Weight, 1)),
		Metadata:    make(map[string]string, 11),
	}

	param.Metadata[metaFieldID] = ins.ID
	param.Metadata[metaFieldName] = ins.Name
	param.Metadata[metaFieldKind] = ins.Kind
	param.Metadata[metaFieldAlias] = ins.Alias
	param.Metadata[metaFieldState] = ins.State
	param.Metadata[metaFieldEndpoint] = ins.Endpoint
	param.Metadata[metaFieldWeight] = xconv.String(int(param.Weight))

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

// Deregister deregisters a service instance.
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

// Watch watches the instance changes of the service with the given name.
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

// doBuildWatcherMgr returns the watch manager for serviceName.
func (r *Registry) doBuildWatcherMgr(_ context.Context, serviceName string) (*watcherMgr, error) {
	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		return mgr, nil
	}

	v, err, _ := r.group.Do(serviceName, func() (any, error) {
		if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
			return mgr, nil
		}

		services, err := r.services(context.Background(), serviceName)
		if err != nil {
			return nil, err
		}

		mgr := newWatcherMgr(r, serviceName, services)
		if err = mgr.init(); err != nil {
			return nil, err
		}

		r.watchers.Store(serviceName, mgr)

		// Store must not register an already closed watch manager when Close and Watch run
		// concurrently.
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

// loadWatcherMgr loads the watch manager for serviceName.
//
// It only returns a manager that has not stopped, so that a stopped manager that has not yet been
// removed from the registry is not returned.
func (r *Registry) loadWatcherMgr(serviceName string) *watcherMgr {
	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && !mgr.stopped.Load() {
			return mgr
		}
	}

	return nil
}

// Services returns the instances of the service with the given name.
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

// Close closes service registry and discovery.
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
		r.opts.client.CloseClient()
	}

	return nil
}

// services returns the instances of the service with the given name.
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
