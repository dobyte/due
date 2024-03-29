package consul

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/symsimmy/due/encoding/json"
	"github.com/symsimmy/due/env"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/registry"
	"net"
	"net/url"
	"strconv"
	"time"
)

const (
	checkIDFormat     = "dispatcher:%s"
	checkUpdateOutput = "passed"
	metaFieldKind     = "kind"
	metaFieldAlias    = "alias"
	metaFieldState    = "state"
)

const (
	routeKvFormat = "route/%v/%v"
	hostIpEnvName = "MY_HOST_IP"
)

type registrar struct {
	ctx         context.Context
	cancel      context.CancelFunc
	registry    *Registry
	chHeartbeat chan string
}

func newRegistrar(registry *Registry) *registrar {
	r := &registrar{}
	r.ctx, r.cancel = context.WithCancel(registry.ctx)
	r.registry = registry
	r.chHeartbeat = make(chan string)

	if r.registry.opts.enableHeartbeatCheck {
		go r.keepHeartbeat()
	}

	return r
}

// 注册服务
func (r *registrar) register(ctx context.Context, ins *registry.ServiceInstance) error {
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

	overwriteHost := env.Get(hostIpEnvName, host).String()

	registration := &api.AgentServiceRegistration{
		ID:      ins.ID,
		Name:    ins.Name,
		Tags:    make([]string, 0, len(ins.Events)),
		Meta:    make(map[string]string, 3),
		Address: overwriteHost,
		Port:    port,
		TaggedAddresses: map[string]api.ServiceAddress{raw.Scheme: {
			Address: overwriteHost,
			Port:    port,
		}},
	}

	registration.Meta[metaFieldKind] = string(ins.Kind)
	registration.Meta[metaFieldAlias] = ins.Alias
	registration.Meta[metaFieldState] = string(ins.State)

	for _, event := range ins.Events {
		registration.Tags = append(registration.Tags, strconv.Itoa(int(event)))
	}

	if r.registry.opts.enableHealthCheck {
		registration.Checks = append(registration.Checks, &api.AgentServiceCheck{
			TCP:                            raw.Host,
			Interval:                       fmt.Sprintf("%ds", r.registry.opts.healthCheckInterval),
			Timeout:                        fmt.Sprintf("%ds", r.registry.opts.healthCheckTimeout),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.registry.opts.deregisterCriticalServiceAfter),
			FailuresBeforeCritical:         r.registry.opts.failuresBeforeCritical,
		})
	}

	if r.registry.opts.enableHeartbeatCheck {
		registration.Checks = append(registration.Checks, &api.AgentServiceCheck{
			CheckID:                        fmt.Sprintf(checkIDFormat, ins.ID),
			TTL:                            fmt.Sprintf("%ds", r.registry.opts.heartbeatCheckInterval),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.registry.opts.deregisterCriticalServiceAfter),
			FailuresBeforeCritical:         r.registry.opts.failuresBeforeCritical,
		})
	}

	if err = r.registry.opts.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	// register route in key/value
	m := make(map[int32]bool, len(ins.Routes))
	for _, route := range ins.Routes {
		m[route.ID] = route.Stateful
	}
	routes, _ := json.Marshal(m)
	ops := &api.KVPair{
		Key:   fmt.Sprintf(routeKvFormat, ins.Alias, ins.ID),
		Value: routes,
	}
	if _, err = r.registry.opts.client.KV().Put(ops, nil); err != nil {
		return err
	}

	if r.registry.opts.enableHeartbeatCheck {
		r.chHeartbeat <- ins.ID
	}

	//注册metrics服务
	if r.registry.opts.metricsEnable {
		metricsInsId := ins.ID + "-exporter"
		metricsRegistration := &api.AgentServiceRegistration{
			ID:      metricsInsId,
			Name:    ins.Name + "-exporter",
			Address: overwriteHost,
			Port:    ins.MetricsPort,
			TaggedAddresses: map[string]api.ServiceAddress{raw.Scheme: {
				Address: overwriteHost,
				Port:    ins.MetricsPort,
			}},
		}

		if r.registry.opts.enableHealthCheck {
			metricsRegistration.Checks = append(metricsRegistration.Checks, &api.AgentServiceCheck{
				TCP:                            raw.Host,
				Interval:                       fmt.Sprintf("%ds", r.registry.opts.healthCheckInterval),
				Timeout:                        fmt.Sprintf("%ds", r.registry.opts.healthCheckTimeout),
				DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.registry.opts.deregisterCriticalServiceAfter),
				FailuresBeforeCritical:         r.registry.opts.failuresBeforeCritical,
			})
		}

		if r.registry.opts.enableHeartbeatCheck {
			metricsRegistration.Checks = append(metricsRegistration.Checks, &api.AgentServiceCheck{
				CheckID:                        fmt.Sprintf(checkIDFormat, metricsInsId),
				TTL:                            fmt.Sprintf("%ds", r.registry.opts.heartbeatCheckInterval),
				DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.registry.opts.deregisterCriticalServiceAfter),
				FailuresBeforeCritical:         r.registry.opts.failuresBeforeCritical,
			})
		}

		if err = r.registry.opts.client.Agent().ServiceRegister(metricsRegistration); err != nil {
			return err
		}

		if r.registry.opts.enableHeartbeatCheck {
			r.chHeartbeat <- metricsInsId
		}
	}

	return nil
}

// 解注册服务
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	r.cancel()
	close(r.chHeartbeat)

	r.registry.registrars.Delete(ins.ID)

	if err := r.registry.opts.client.Agent().ServiceDeregister(ins.ID); err != nil {
		return err
	}

	//解注册metrics服务
	if r.registry.opts.metricsEnable {
		if err := r.registry.opts.client.Agent().ServiceDeregister(ins.ID + "-exporter"); err != nil {
			return err
		}
	}

	// deregister route in key/value
	if _, err := r.registry.opts.client.KV().Delete(fmt.Sprintf(routeKvFormat, ins.Alias, ins.ID), nil); err != nil {
		return err
	}

	return nil
}

// 心跳检测
func (r *registrar) keepHeartbeat() {
	var (
		ctxSlice    []context.Context
		cancelSlice []context.CancelFunc
	)

	for {
		select {
		case insID, ok := <-r.chHeartbeat:
			if !ok {
				return
			}

			ctx, cancel := context.WithCancel(r.ctx)
			ctxSlice = append(ctxSlice, ctx)
			cancelSlice = append(cancelSlice, cancel)

			go r.heartbeat(ctx, insID)
		case <-r.ctx.Done():
			for _, cancel := range cancelSlice {
				cancel()
			}
			return
		}
	}
}

// 心跳
func (r *registrar) heartbeat(ctx context.Context, insID string) {
	checkID := fmt.Sprintf(checkIDFormat, insID)

	err := r.registry.opts.client.Agent().UpdateTTL(checkID, checkUpdateOutput, api.HealthPassing)
	if err != nil {
		log.Errorf("update heartbeat ttl failed: %v", err)
	}

	ticker := time.NewTicker(time.Duration(r.registry.opts.heartbeatCheckInterval) * time.Second / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}

			if err = r.registry.opts.client.Agent().UpdateTTL(checkID, checkUpdateOutput, api.HealthPassing); err != nil {
				log.Errorf("update heartbeat ttl failed: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
