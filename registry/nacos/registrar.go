package nacos

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

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

type registrar struct {
	registry *Registry
	mu       sync.Mutex
	stopped  atomic.Bool
	wg       sync.WaitGroup
	ins      *registry.ServiceInstance
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

	host, port, err := r.parseHostPort(ins.Endpoint)
	if err != nil {
		return err
	}

	ok, err := r.put(ctx, ins, host, port)
	if err != nil {
		return err
	}

	if !ok {
		return errors.ErrServiceRegisterFailed
	}

	r.mu.Lock()

	if r.stopped.Load() {
		r.mu.Unlock()
		r.delete(ctx, ins, host, port)
		return errors.ErrIllegalOperation
	}

	r.ins = ins
	r.mu.Unlock()

	return nil
}

func (r *registrar) deregister(_ context.Context, ins *registry.ServiceInstance) error {
	host, port, err := r.parseHostPort(ins.Endpoint)
	if err != nil {
		return err
	}

	ok, err := r.registry.opts.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: ins.Name,
		Cluster:     r.registry.opts.clusterName,
		GroupName:   r.registry.opts.groupName,
		Ephemeral:   true,
	})
	if err != nil {
		return err
	}

	if !ok {
		return errors.ErrServiceDeregisterFailed
	}

	r.stop()

	return nil
}

func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	r.wg.Wait()
}

func (r *registrar) put(_ context.Context, ins *registry.ServiceInstance, host string, port uint64) (bool, error) {
	param := vo.RegisterInstanceParam{
		Ip:          host,
		Port:        port,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		ServiceName: ins.Name,
		ClusterName: r.registry.opts.clusterName,
		GroupName:   r.registry.opts.groupName,
		Metadata:    make(map[string]string, 11),
	}

	param.Metadata[metaFieldID] = ins.ID
	param.Metadata[metaFieldName] = ins.Name
	param.Metadata[metaFieldKind] = ins.Kind
	param.Metadata[metaFieldAlias] = ins.Alias
	param.Metadata[metaFieldState] = ins.State
	param.Metadata[metaFieldEndpoint] = ins.Endpoint

	if ins.Weight > 0 {
		param.Weight = float64(ins.Weight)
		param.Metadata[metaFieldWeight] = xconv.String(ins.Weight)
	} else {
		param.Weight = 1
	}

	if len(ins.Routes) > 0 {
		if routes, err := json.Marshal(ins.Routes); err != nil {
			return false, err
		} else {
			param.Metadata[metaFieldRoutes] = xconv.BytesToString(routes)
		}
	}

	if len(ins.Events) > 0 {
		if events, err := json.Marshal(ins.Events); err != nil {
			return false, err
		} else {
			param.Metadata[metaFieldEvents] = xconv.BytesToString(events)
		}
	}

	if len(ins.Services) > 0 {
		if services, err := json.Marshal(ins.Services); err != nil {
			return false, err
		} else {
			param.Metadata[metaFieldServices] = xconv.BytesToString(services)
		}
	}

	if len(ins.Metadata) > 0 {
		if metadata, err := json.Marshal(ins.Metadata); err != nil {
			return false, err
		} else {
			param.Metadata[metaFieldMetadata] = xconv.BytesToString(metadata)
		}
	}

	return r.registry.opts.client.RegisterInstance(param)
}

func (r *registrar) delete(_ context.Context, ins *registry.ServiceInstance, host string, port uint64) {
	if _, err := r.registry.opts.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: ins.Name,
		Cluster:     r.registry.opts.clusterName,
		GroupName:   r.registry.opts.groupName,
		Ephemeral:   true,
	}); err != nil {
		log.Errorf("deregister instance failed: %v", err)
	}
}

func (r *registrar) parseHostPort(endpoint string) (string, uint64, error) {
	raw, err := url.Parse(endpoint)
	if err != nil {
		return "", 0, err
	}

	host, p, err := net.SplitHostPort(raw.Host)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.ParseUint(p, 10, 64)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}
