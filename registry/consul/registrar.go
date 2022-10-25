package consul

import (
	"context"
	"fmt"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"github.com/hashicorp/consul/api"
	"net"
	"net/url"
	"strconv"
	"time"
)

const (
	checkIDFormat     = "service:%s"
	checkUpdateOutput = "passed"
	metaFieldKind     = "kind"
	metaFieldAlias    = "alias"
	metaFieldState    = "state"
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

	registration := &api.AgentServiceRegistration{
		ID:      ins.ID,
		Name:    ins.Name,
		Meta:    make(map[string]string, len(ins.Routes)+3),
		Address: host,
		Port:    port,
		TaggedAddresses: map[string]api.ServiceAddress{raw.Scheme: {
			Address: host,
			Port:    port,
		}},
	}

	registration.Meta[metaFieldKind] = string(ins.Kind)
	registration.Meta[metaFieldAlias] = ins.Alias
	registration.Meta[metaFieldState] = string(ins.State)
	for _, route := range ins.Routes {
		registration.Meta[strconv.Itoa(int(route.ID))] = strconv.FormatBool(route.Stateful)
	}

	if r.registry.opts.enableHealthCheck {
		registration.Checks = append(registration.Checks, &api.AgentServiceCheck{
			TCP:                            raw.Host,
			Interval:                       fmt.Sprintf("%ds", r.registry.opts.healthCheckInterval),
			Timeout:                        fmt.Sprintf("%ds", r.registry.opts.healthCheckTimeout),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.registry.opts.deregisterCriticalServiceAfter),
		})
	}

	if r.registry.opts.enableHeartbeatCheck {
		registration.Checks = append(registration.Checks, &api.AgentServiceCheck{
			CheckID:                        fmt.Sprintf(checkIDFormat, ins.ID),
			TTL:                            fmt.Sprintf("%ds", r.registry.opts.heartbeatCheckInterval),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.registry.opts.deregisterCriticalServiceAfter),
		})
	}

	if err = r.registry.opts.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	if r.registry.opts.enableHeartbeatCheck {
		r.chHeartbeat <- ins.ID
	}

	return nil
}

// 解注册服务
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	r.cancel()
	close(r.chHeartbeat)

	r.registry.registrars.Delete(ins.ID)

	return r.registry.opts.client.Agent().ServiceDeregister(ins.ID)
}

// 心跳检测
func (r *registrar) keepHeartbeat() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	for {
		select {
		case insID, ok := <-r.chHeartbeat:
			if cancel != nil {
				cancel()
			}

			if !ok {
				return
			}

			ctx, cancel = context.WithCancel(r.ctx)
			go r.heartbeat(ctx, insID)
		case <-r.ctx.Done():
			if cancel != nil {
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
		log.Warnf("update heartbeat ttl failed: %v", err)
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
				log.Warnf("update heartbeat ttl failed: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
