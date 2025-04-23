package consul

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/hashicorp/consul/api"
	"net"
	"net/url"
	"strconv"
	"time"
)

const (
	checkIDFormat     = "service:%s"
	checkUpdateOutput = "passed"
	metaFieldID       = "id"
	metaFieldKind     = "kind"
	metaFieldAlias    = "alias"
	metaFieldState    = "state"
	metaFieldRoutes   = "routes"
	metaFieldEvents   = "events"
	metaFieldWeight   = "weight"
	metaFieldServices = "services"
	metaFieldEndpoint = "endpoint"
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

	insID := makeInsID(ins)

	registration := &api.AgentServiceRegistration{}
	registration.ID = insID
	registration.Name = ins.Name
	registration.Address = host
	registration.Port = port
	registration.TaggedAddresses = map[string]api.ServiceAddress{raw.Scheme: {Address: host, Port: port}}
	registration.Meta = make(map[string]string, 7)
	registration.Meta[metaFieldID] = ins.ID
	registration.Meta[metaFieldKind] = ins.Kind
	registration.Meta[metaFieldAlias] = ins.Alias
	registration.Meta[metaFieldState] = ins.State
	registration.Meta[metaFieldEndpoint] = ins.Endpoint
	registration.Meta[metaFieldEvents] = xconv.Json(ins.Events)
	registration.Meta[metaFieldWeight] = xconv.String(ins.Weight)
	registration.Meta[metaFieldServices] = xconv.Json(ins.Services)

	for field, value := range marshalMetaRoutes(ins.Routes) {
		registration.Meta[field] = value
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
			CheckID:                        fmt.Sprintf(checkIDFormat, insID),
			TTL:                            fmt.Sprintf("%ds", r.registry.opts.heartbeatCheckInterval),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", r.registry.opts.deregisterCriticalServiceAfter),
		})
	}

	if err = r.registry.opts.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	if r.registry.opts.enableHeartbeatCheck {
		r.chHeartbeat <- insID
	}

	return nil
}

// 解注册服务
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	r.cancel()
	close(r.chHeartbeat)

	return r.registry.opts.client.Agent().ServiceDeregister(makeInsID(ins))
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
