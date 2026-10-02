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

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/hashicorp/consul/api"
)

// Constants related to heartbeat checks.
const (
	checkIDFormat     = "service:%s" // Heartbeat check ID format
	checkUpdateOutput = "passed"     // Heartbeat update output
)

// registrar is a service registrar responsible for registering service instances, keeping them
// alive via heartbeats and deregistering them.
type registrar struct {
	registry *Registry          // Owning registry and discovery component
	ctx      context.Context    // Heartbeat goroutine context
	cancel   context.CancelFunc // Heartbeat goroutine cancel function
	insID    string             // Unique instance identifier
	mu       sync.Mutex         // Protects the ctx and cancel fields
	stopped  atomic.Bool        // Whether the registrar has stopped
	wg       sync.WaitGroup     // Waits for the heartbeat goroutine to exit
}

// newRegistrar creates a service registrar.
func newRegistrar(registry *Registry, insID string) *registrar {
	r := &registrar{}
	r.registry = registry
	r.insID = insID

	return r
}

// register registers a service instance and, when needed, starts the heartbeat keep-alive
// goroutine.
func (r *registrar) register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.stopped.Load() {
		return errors.ErrIllegalOperation
	}

	tctx, tcancel := context.WithTimeout(ctx, r.registry.opts.timeout)
	insID, err := r.put(tctx, ins)
	tcancel()
	if err != nil {
		return err
	}

	r.mu.Lock()

	if r.stopped.Load() {
		r.mu.Unlock()
		if err := r.deregisterService(context.Background(), insID); err != nil {
			log.Warnf("deregister service %s failed: %v", insID, err)
		}
		return errors.ErrIllegalOperation
	}

	oldCancel := r.cancel
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	r.cancel = cancel

	if r.registry.opts.enableHeartbeatCheck {
		r.wg.Add(1)
	}

	r.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	if r.registry.opts.enableHeartbeatCheck {
		go r.heartbeat(ctx, ins)
	}

	return nil
}

// deregister deregisters a service instance, stops the heartbeat and deregisters the service.
func (r *registrar) deregister(ctx context.Context) error {
	r.stop()

	return r.deregisterService(ctx, r.insID)
}

// stop stops registration.
//
// It only stops the heartbeat and waits for the goroutine to exit; it does not deregister the
// service.
func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	r.cleanup()

	r.wg.Wait()
}

// close closes registration.
//
// It proactively deregisters the service, stops the heartbeat and waits for the goroutine to exit.
func (r *registrar) close() {
	r.stop()

	if err := r.deregisterService(context.Background(), r.insID); err != nil {
		log.Warnf("deregister service %s failed: %v", r.insID, err)
	}
}

// cleanup releases the registration resources.
//
// It cancels the heartbeat and removes the registrar without waiting for the heartbeat goroutine to
// exit. It is called by the heartbeat goroutine itself to avoid waiting on itself.
func (r *registrar) cleanup() {
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	r.registry.registrars.Delete(r.insID)
}

// put registers a service instance with Consul and returns the unique instance identifier.
func (r *registrar) put(ctx context.Context, ins *registry.ServiceInstance) (string, error) {
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
		metas, err := marshalMetaList(metaFieldEvents, ins.Events)
		if err != nil {
			return "", err
		}

		for field, value := range metas {
			registration.Meta[field] = value
		}
	}

	if len(ins.Services) > 0 {
		metas, err := marshalMetaList(metaFieldServices, ins.Services)
		if err != nil {
			return "", err
		}

		for field, value := range metas {
			registration.Meta[field] = value
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

	if err = r.registry.opts.client.Agent().ServiceRegisterOpts(registration, api.ServiceRegisterOpts{}.WithContext(ctx)); err != nil {
		return "", err
	}

	return insID, nil
}

// deregisterService deregisters from Consul the service with the given instance identifier.
func (r *registrar) deregisterService(ctx context.Context, insID string) error {
	tctx, tcancel := context.WithTimeout(ctx, r.registry.opts.timeout)
	defer tcancel()

	if err := r.registry.opts.client.Agent().ServiceDeregisterOpts(insID, (&api.QueryOptions{}).WithContext(tctx)); err != nil {
		return err
	}

	return nil
}

// heartbeat keeps the service alive.
//
// When a heartbeat update fails, it attempts to re-register during the backoff retries to heal
// itself. Once the continuous failure duration exceeds the deregisterCriticalServiceAfter
// threshold, the registration is considered lost and maintenance is stopped and cleaned up. When
// deregisterCriticalServiceAfter is less than or equal to 0, meaning Consul never automatically
// deregisters the service, the heartbeat keeps retrying instead of giving up.
func (r *registrar) heartbeat(ctx context.Context, ins *registry.ServiceInstance) {
	defer r.wg.Done()

	if ctx.Err() != nil {
		return
	}

	insID := makeInsID(ins)
	checkID := fmt.Sprintf(checkIDFormat, insID)
	critical := time.Duration(r.registry.opts.deregisterCriticalServiceAfter) * time.Second

	var (
		ok        bool
		failureAt time.Time
	)

	if err := r.updateTTL(ctx, checkID); err == nil {
		ok = true
	}

	interval := time.Duration(r.registry.opts.heartbeatCheckInterval) * time.Second / 2
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		if !ok {
			if failureAt.IsZero() {
				failureAt = time.Now()
				log.Warnf("consul heartbeat failed, retry to register service %s", insID)
			}

			err := xcall.Backoff(ctx, func(ctx context.Context, attempt int) (bool, error) {
				tctx, tcancel := context.WithTimeout(ctx, r.registry.opts.timeout)
				_, err := r.put(tctx, ins)
				tcancel()

				if err != nil {
					return true, err
				}

				return false, nil
			}, max(1, r.registry.opts.retryTimes), 100*time.Millisecond, time.Second)

			if err == nil {
				// Re-registration succeeded; the service has healed itself.
				failureAt = time.Time{}
				ok = true
			} else if critical > 0 && time.Since(failureAt) >= critical {
				// Continuous failures exceeded the auto-deregistration threshold; the registration
				// is unrecoverable, so give up maintenance.
				r.mu.Lock()
				if r.ctx != ctx {
					// It has been superseded by a new registration; return without affecting the
					// new registration.
					r.mu.Unlock()
					return
				}

				if !r.stopped.CompareAndSwap(false, true) {
					r.mu.Unlock()
					return
				}
				r.mu.Unlock()

				r.cleanup()

				log.Errorf("consul heartbeat failed, service registration lost")

				return
			}
		}

		select {
		case <-timer.C:
			if ctx.Err() != nil {
				return
			}

			if err := r.updateTTL(ctx, checkID); err != nil {
				if failureAt.IsZero() {
					failureAt = time.Now()
					log.Warnf("update heartbeat ttl failed: %v", err)
				}

				ok = false
			} else {
				failureAt = time.Time{}
				ok = true
			}

			timer.Reset(interval)
		case <-ctx.Done():
			return
		}
	}
}

// updateTTL updates the service's health check heartbeat.
func (r *registrar) updateTTL(ctx context.Context, checkID string) error {
	tctx, cancel := context.WithTimeout(ctx, r.registry.opts.timeout)
	defer cancel()

	return r.registry.opts.client.Agent().UpdateTTLOpts(checkID, checkUpdateOutput, api.HealthPassing, (&api.QueryOptions{}).WithContext(tctx))
}
