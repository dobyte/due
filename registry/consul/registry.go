// Package consul provides a Consul-based service registry and discovery component that implements
// the [registry.Registry] interface. It supports registering, deregistering, watching and querying
// service instances, as well as health checks and heartbeat keep-alive.
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

// name is the name of the service registry and discovery component.
const name = "consul"

var _ registry.Registry = &Registry{}

// Registry is a Consul-based service registry and discovery component that implements
// [registry.Registry].
type Registry struct {
	err        error      // Initialization error, recorded when creating the built-in client fails
	opts       *options   // Options
	mu1        sync.Mutex // Protects the watchers registry
	watchers   sync.Map   // Registry of service watcher managers
	mu2        sync.Mutex // Protects the registrars registry
	registrars sync.Map   // Registry of service registrars
}

// NewRegistry creates a Consul-based service registry and discovery instance.
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

// Name returns the name of the service registry and discovery component.
func (r *Registry) Name() string {
	return name
}

// Register registers a service instance.
func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	return r.doBuildRegistrar(makeInsID(ins)).register(ctx, ins)
}

// doBuildRegistrar builds a service registrar.
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

// Deregister deregisters a service instance.
func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if v, ok := r.registrars.Load(makeInsID(ins)); ok {
		return v.(*registrar).deregister(ctx)
	}

	return nil
}

// Watch watches for changes to the service instances with the same service name.
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

// doBuildWatcherMgr builds a service watcher manager.
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

// loadWatcherMgr loads the service watcher manager, returning nil when it does not exist or has
// stopped.
func (r *Registry) loadWatcherMgr(serviceName string) *watcherMgr {
	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && !mgr.stopped.Load() {
			return mgr
		}
	}

	return nil
}

// Close closes the service registry and discovery component.
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

// Services returns the list of service instances.
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

// services queries from Consul the healthy instance list of the given service and returns the
// latest index.
func (r *Registry) services(ctx context.Context, serviceName string, waitIndex uint64, passingOnly, blocking bool) ([]*registry.ServiceInstance, uint64, error) {
	opts := &api.QueryOptions{
		WaitIndex: waitIndex,
	}

	// A blocking query needs WaitTime to be set; a non-blocking query returns immediately.
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

		// Skip service instances not registered by the due framework (missing the ID metadata) to
		// avoid mixing in invalid instances.
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
