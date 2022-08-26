package consul

import (
	"context"
	"fmt"
	"github.com/dobyte/due/log"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/dobyte/due/registry"
)

const (
	checkIDFormat     = "service:%s"
	checkUpdateOutput = "passed"
)

type Registry struct {
	err    error
	ctx    context.Context
	cancel context.CancelFunc
	client *api.Client
	opts   *options

	mu       sync.Mutex
	watchers map[string]*watcher
}

func NewRegistry(opts ...Option) *Registry {
	o := &options{
		ctx:                            context.Background(),
		address:                        "127.0.0.1:8500",
		enableHealthCheck:              true,
		healthCheckInterval:            10,
		healthCheckTimeout:             5,
		enableHeartbeatCheck:           true,
		heartbeatCheckInterval:         10,
		deregisterCriticalServiceAfter: 30,
	}
	for _, opt := range opts {
		opt(o)
	}

	config := api.DefaultConfig()
	if o.address != "" {
		config.Address = o.address
	}

	r := &Registry{}
	r.opts = o
	r.watchers = make(map[string]*watcher)
	r.ctx, r.cancel = context.WithCancel(o.ctx)
	r.client, r.err = api.NewClient(config)

	return r
}

// Register 注册服务实例
func (r *Registry) Register(ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	raw, err := url.Parse(ins.Endpoint)
	if err != nil {
		return err
	}

	host, p, err := net.SplitHostPort(raw.Host)
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		return err
	}

	registration := &api.AgentServiceRegistration{
		ID:      ins.ID,
		Name:    ins.Name,
		Meta:    make(map[string]string, len(ins.Routes)),
		Address: host,
		Port:    port,
		TaggedAddresses: map[string]api.ServiceAddress{raw.Scheme: {
			Address: host,
			Port:    port,
		}},
	}

	for _, route := range ins.Routes {
		registration.Meta[strconv.Itoa(int(route.ID))] = strconv.FormatBool(route.Stateful)
	}

	if r.opts.enableHealthCheck {
		registration.Checks = append(registration.Checks, &api.AgentServiceCheck{
			TCP:                            raw.Host,
			Interval:                       fmt.Sprintf("%ds", r.opts.healthCheckInterval),
			Timeout:                        fmt.Sprintf("%ds", r.opts.healthCheckTimeout),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.opts.deregisterCriticalServiceAfter),
		})
	}

	if r.opts.enableHeartbeatCheck {
		registration.Checks = append(registration.Checks, &api.AgentServiceCheck{
			CheckID:                        fmt.Sprintf(checkIDFormat, ins.ID),
			TTL:                            fmt.Sprintf("%ds", r.opts.heartbeatCheckInterval),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.opts.deregisterCriticalServiceAfter),
		})
	}

	if err = r.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	if r.opts.enableHeartbeatCheck {
		go r.heartbeat(ins.ID)
	}

	return nil
}

// Deregister 解注册服务实例
func (r *Registry) Deregister(ins *registry.ServiceInstance) error {
	r.cancel()
	return r.client.Agent().ServiceDeregister(ins.ID)
}

// Services 获取服务实例列表
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if w, ok := r.watchers[serviceName]; ok {
		if services := w.services(); len(services) > 0 {
			return services, nil
		}
	}

	services, _, err := r.services(ctx, serviceName, 0, true)

	return services, err
}

// Watch 监听服务
func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	w, ok := r.watchers[serviceName]
	if !ok {
		w = newWatcher(r.ctx, serviceName)
		r.watchers[serviceName] = w
	}

	if err := r.resolve(ctx, w); err != nil {
		return nil, err
	}

	return w, nil
}

func (r *Registry) resolve(ctx context.Context, w *watcher) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	services, index, err := r.services(ctx, w.serviceName, 0, true)
	cancel()
	if err != nil {
		return err
	}
	w.update(services)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
				services, tmpIndex, err := r.services(ctx, w.serviceName, index, true)
				cancel()
				if err != nil {
					time.Sleep(time.Second)
					continue
				}
				if index != tmpIndex {
					w.update(services)
					index = tmpIndex
				}
			}
		}
	}()

	return nil
}

// 获取服务实体列表
func (r *Registry) services(ctx context.Context, serviceName string, waitIndex uint64, passingOnly bool) ([]*registry.ServiceInstance, uint64, error) {
	opts := &api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  60 * time.Second,
	}
	opts.WithContext(ctx)

	entries, meta, err := r.client.Health().Service(serviceName, "", passingOnly, opts)
	if err != nil {
		return nil, 0, err
	}

	services := make([]*registry.ServiceInstance, 0, len(entries))
	for _, entry := range entries {
		routes := make([]registry.Route, 0, len(entry.Service.Meta))
		for k, v := range entry.Service.Meta {
			route, err := strconv.Atoi(k)
			if err != nil {
				continue
			}

			stateful, err := strconv.ParseBool(v)
			if err != nil {
				continue
			}

			routes = append(routes, registry.Route{
				ID:       int32(route),
				Stateful: stateful,
			})
		}

		var endpoint string
		for scheme, addr := range entry.Service.TaggedAddresses {
			if scheme == "lan_ipv4" || scheme == "wan_ipv4" || scheme == "lan_ipv6" || scheme == "wan_ipv6" {
				continue
			}
			endpoint = (&url.URL{
				Scheme: scheme,
				Host:   net.JoinHostPort(addr.Address, strconv.Itoa(addr.Port)),
			}).String()
		}
		if endpoint == "" {
			continue
		}

		services = append(services, &registry.ServiceInstance{
			ID:       entry.Service.ID,
			Name:     entry.Service.Service,
			Routes:   routes,
			Endpoint: endpoint,
		})
	}

	return services, meta.LastIndex, nil
}

// 心跳
func (r *Registry) heartbeat(insID string) {
	time.Sleep(time.Second)

	checkID := fmt.Sprintf(checkIDFormat, insID)

	err := r.client.Agent().UpdateTTL(checkID, checkUpdateOutput, api.HealthPassing)
	if err != nil {
		log.Errorf("update heartbeat ttl failed: %v", err)
	}

	ticker := time.NewTicker(time.Duration(r.opts.heartbeatCheckInterval) * time.Second / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err = r.client.Agent().UpdateTTL(checkID, checkUpdateOutput, api.HealthPassing); err != nil {
				log.Errorf("update heartbeat ttl failed: %v", err)
			}
		case <-r.ctx.Done():
			return
		}
	}
}
