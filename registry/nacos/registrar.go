package nacos

import (
	"context"
	"net"
	"net/url"
	"strconv"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
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
}

func newRegistrar(registry *Registry) *registrar {
	return &registrar{registry: registry}
}

// 注册服务
func (r *registrar) register(_ context.Context, ins *registry.ServiceInstance) error {
	host, port, err := r.parseHostPort(ins.Endpoint)
	if err != nil {
		return err
	}

	param := vo.RegisterInstanceParam{
		Ip:          host,
		Port:        port,
		Weight:      1,
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
		param.Metadata[metaFieldWeight] = xconv.String(ins.Weight)
	}

	if len(ins.Routes) > 0 {
		if routes, err := json.Marshal(ins.Routes); err != nil {
			return err
		} else {
			param.Metadata[metaFieldRoutes] = xconv.BytesToString(routes)
		}
	}

	if len(ins.Events) > 0 {
		if events, err := json.Marshal(ins.Events); err != nil {
			return err
		} else {
			param.Metadata[metaFieldEvents] = xconv.BytesToString(events)
		}
	}

	if len(ins.Services) > 0 {
		if services, err := json.Marshal(ins.Services); err != nil {
			return err
		} else {
			param.Metadata[metaFieldServices] = xconv.BytesToString(services)
		}
	}

	if len(ins.Metadata) > 0 {
		if metadata, err := json.Marshal(ins.Metadata); err != nil {
			return err
		} else {
			param.Metadata[metaFieldMetadata] = xconv.BytesToString(metadata)
		}
	}

	ok, err := r.registry.opts.client.RegisterInstance(param)
	if err != nil {
		return err
	}

	if !ok {
		return errors.ErrServiceRegisterFailed
	}

	return nil
}

// 解注册服务
func (r *registrar) deregister(_ context.Context, ins *registry.ServiceInstance) error {
	host, port, err := r.parseHostPort(ins.Endpoint)
	if err != nil {
		return err
	}

	param := vo.DeregisterInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: ins.Name,
		Cluster:     r.registry.opts.clusterName,
		GroupName:   r.registry.opts.groupName,
		Ephemeral:   true,
	}

	ok, err := r.registry.opts.client.DeregisterInstance(param)
	if err != nil {
		return err
	}

	if !ok {
		return errors.ErrServiceDeregisterFailed
	}

	return nil
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
