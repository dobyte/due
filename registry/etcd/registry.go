// Package etcd provides an etcd-backed implementation of service registry and discovery.
//
// It supports registering and deregistering service instances, watching the instance changes of a
// service, and querying service instances.
package etcd

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const name = "etcd"

var _ registry.Registry = &Registry{}

// Registry is a service registry and discovery component.
//
// It is backed by etcd and supports registering and deregistering service instances, watching the
// instance changes of a service, and querying service instances.
type Registry struct {
	err        error              // Initialization error, recorded when the built-in client fails to be created
	opts       *options           // Configuration options
	builtin    bool               // Whether the built-in client is used
	ctx        context.Context    // Component root context; keepalive and watch goroutines derive from it and terminate with the component
	cancel     context.CancelFunc // Cancel function of the component root context
	mu1        sync.Mutex         // Guards the watchers registry
	watchers   sync.Map           // Registry of service watch managers
	mu2        sync.Mutex         // Guards the registrars registry
	registrars sync.Map           // Registry of service registrars
}

// NewRegistry returns a new service registry and discovery component.
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

// doBuildRegistrar returns the registrar for insID.
//
// The registrar of an instance ID is reused from the registry, so that recreating it does not
// interleave the keepalive streams.
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
		return v.(*registrar).deregister(ctx, ins)
	}

	return nil
}

// Watch watches the instance changes of the service with the given name.
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

// doBuildWatcherMgr returns the watch manager for serviceName.
//
// The watch manager of a service name is reused from the registry and is only rebuilt when it is
// missing or has been stopped.
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

// loadWatcherMgr loads the watch manager for serviceName.
func (r *Registry) loadWatcherMgr(serviceName string) *watcherMgr {
	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && !mgr.stopped.Load() {
			return mgr
		}
	}

	return nil
}

// Services returns the instances of the service with the given name.
//
// It returns cached data while the watch link is healthy and otherwise queries etcd directly, so
// that stale data is not served.
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && mgr.health.Load() {
			if services, err := mgr.services(); err == nil {
				return services, nil
			}
		}
	}

	return r.services(ctx, serviceName)
}

// Close closes service registry and discovery.
//
// It stops every service registrar and watch manager and releases their resources; the client is
// closed as well when it is the built-in one.
func (r *Registry) Close() error {
	if r.err != nil {
		return r.err
	}

	// Cancel the component root context first so that every derived keepalive and watch goroutine
	// terminates with the component.
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

// services returns the instances of the service with the given name.
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
