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

func newRegistrar(registry *Registry, insID string) *registrar {
	r := &registrar{}
	r.registry = registry
	r.insID = insID

	return r
}

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
		r.deregisterService(insID)
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

func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	defer r.stop()

	dctx, cancel := context.WithTimeout(ctx, r.registry.opts.timeout)
	defer cancel()

	if err := r.registry.opts.client.Agent().ServiceDeregisterOpts(makeInsID(ins), (&api.QueryOptions{}).WithContext(dctx)); err != nil {
		return err
	}

	return nil
}

// 停止注册
// 仅停止心跳并等待协程退出，不主动注销服务
func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	r.cleanup()

	r.wg.Wait()
}

// 关闭注册
// 主动注销服务并停止心跳，等待协程退出
func (r *registrar) close() {
	r.deregisterService(r.insID)

	r.stop()
}

// 清理注册资源
// 取消心跳并移除注册器，不等待心跳协程退出（供心跳协程自身调用，避免自等待）
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

func (r *registrar) deregisterService(insID string) {
	dctx, cancel := context.WithTimeout(context.Background(), r.registry.opts.timeout)
	defer cancel()

	if err := r.registry.opts.client.Agent().ServiceDeregisterOpts(insID, (&api.QueryOptions{}).WithContext(dctx)); err != nil {
		log.Warnf("deregister service %s failed: %v", insID, err)
	}
}

// 心跳保活
// 心跳更新失败时，在退避重试中尝试重新注册实现自愈；连续失败时长超过
// deregisterCriticalServiceAfter 阈值后，判定注册已丢失，停止维护并清理
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
			}, r.registry.opts.retryTimes, 100*time.Millisecond, time.Second)

			if err == nil {
				// 重新注册成功，服务已自愈
				failureAt = time.Time{}
				ok = true
			} else if critical <= 0 || time.Since(failureAt) >= critical {
				// 连续失败已超过自动注销阈值，注册不可恢复，放弃维护
				r.mu.Lock()
				current := r.ctx == ctx
				r.mu.Unlock()
				if !current {
					// 已被新的注册取代，直接退出，不影响新注册
					return
				}

				if !r.stopped.CompareAndSwap(false, true) {
					return
				}

				r.cleanup()

				log.Errorf("consul heartbeat failed, service registration lost")

				return
			}
		}

		select {
		case <-time.After(time.Duration(r.registry.opts.heartbeatCheckInterval) * time.Second / 2):
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
		case <-ctx.Done():
			return
		}
	}
}

// 更新服务健康检查心跳
func (r *registrar) updateTTL(ctx context.Context, checkID string) error {
	tctx, cancel := context.WithTimeout(ctx, r.registry.opts.timeout)
	defer cancel()

	return r.registry.opts.client.Agent().UpdateTTLOpts(checkID, checkUpdateOutput, api.HealthPassing, (&api.QueryOptions{}).WithContext(tctx))
}
