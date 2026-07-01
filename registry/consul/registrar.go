package consul

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/hashicorp/consul/api"
)

const (
	checkIDFormat     = "service:%s"
	checkUpdateOutput = "passed"
)

type registrar struct {
	registry *Registry
	ctx      context.Context
	cancel   context.CancelFunc
	insID    string
	mu       sync.Mutex
	stopped  atomic.Bool
	wg       sync.WaitGroup
}

func newRegistrar(registry *Registry) *registrar {
	r := &registrar{}
	r.registry = registry

	return r
}

func (r *registrar) register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.stopped.Load() {
		return errors.ErrIllegalOperation
	}

	insID, err := r.put(ctx, ins)
	if err != nil {
		return err
	}

	r.mu.Lock()

	if r.stopped.Load() {
		r.mu.Unlock()
		r.deregisterService(insID)
		return errors.ErrIllegalOperation
	}

	oldCancel := r.cancel
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.insID = insID

	if r.registry.opts.enableHeartbeatCheck {
		r.wg.Add(1)
	}

	r.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	if r.registry.opts.enableHeartbeatCheck {
		go r.heartbeat(r.ctx, insID)
	}

	return nil
}

func (r *registrar) deregister(_ context.Context, ins *registry.ServiceInstance) error {
	if err := r.registry.opts.client.Agent().ServiceDeregister(makeInsID(ins)); err != nil {
		return err
	}

	r.stop()

	return nil
}

func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	r.wg.Wait()
}

func (r *registrar) put(_ context.Context, ins *registry.ServiceInstance) (string, error) {
	raw, err := url.Parse(ins.Endpoint)
	if err != nil {
		return "", err
	}

	host, p, err := net.SplitHostPort(raw.Host)
	if err != nil {
		return "", err
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		return "", err
	}

	insID := makeInsID(ins)

	registration := &api.AgentServiceRegistration{}
	registration.ID = insID
	registration.Name = ins.Name
	registration.Address = host
	registration.Port = port
	registration.TaggedAddresses = map[string]api.ServiceAddress{raw.Scheme: {Address: host, Port: port}}
	registration.Meta = make(map[string]string, 8)
	registration.Meta[metaFieldID] = ins.ID
	registration.Meta[metaFieldKind] = ins.Kind
	registration.Meta[metaFieldAlias] = ins.Alias
	registration.Meta[metaFieldState] = ins.State
	registration.Meta[metaFieldEndpoint] = ins.Endpoint

	if ins.Weight > 0 {
		registration.Meta[metaFieldWeight] = xconv.String(ins.Weight)
	}

	if len(ins.Events) > 0 {
		if events, err := json.Marshal(ins.Events); err != nil {
			return "", err
		} else {
			registration.Meta[metaFieldEvents] = xconv.BytesToString(events)
		}
	}

	if len(ins.Services) > 0 {
		if services, err := json.Marshal(ins.Services); err != nil {
			return "", err
		} else {
			registration.Meta[metaFieldServices] = xconv.BytesToString(services)
		}
	}

	for field, value := range marshalMetaRoutes(ins.Routes) {
		registration.Meta[field] = value
	}

	for field, value := range ins.Metadata {
		registration.Meta[defaultMetadataPrefix+field] = value
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
		return "", err
	}

	return insID, nil
}

func (r *registrar) deregisterService(insID string) {
	if err := r.registry.opts.client.Agent().ServiceDeregister(insID); err != nil {
		log.Warnf("deregister service %s failed: %v", insID, err)
	}
}

func (r *registrar) heartbeat(ctx context.Context, insID string) {
	defer r.wg.Done()

	checkID := fmt.Sprintf(checkIDFormat, insID)

	err := r.registry.opts.client.Agent().UpdateTTL(checkID, checkUpdateOutput, api.HealthPassing)
	ok := err == nil

	for {
		if !ok {
			for i := 0; i < r.registry.opts.retryTimes; i++ {
				if ctx.Err() != nil {
					return
				}

				err = r.registry.opts.client.Agent().UpdateTTL(checkID, checkUpdateOutput, api.HealthPassing)

				if err != nil {
					select {
					case <-time.After(r.registry.opts.retryInterval):
						log.Warnf("consul heartbeat failed, retry %d times, err: %v", i+1, err)
					case <-ctx.Done():
						return
					}
				} else {
					ok = true
					break
				}
			}

			if !ok {
				log.Errorf("consul heartbeat failed after %d retries", r.registry.opts.retryTimes)
				return
			}
		}

		select {
		case <-time.After(time.Duration(r.registry.opts.heartbeatCheckInterval) * time.Second / 2):
			if ctx.Err() != nil {
				return
			}

			if err = r.registry.opts.client.Agent().UpdateTTL(checkID, checkUpdateOutput, api.HealthPassing); err != nil {
				log.Warnf("update heartbeat ttl failed: %v", err)
				ok = false
			}
		case <-ctx.Done():
			return
		}
	}
}
