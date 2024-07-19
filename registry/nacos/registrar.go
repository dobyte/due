package nacos

import (
	"context"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"net"
	"net/url"
	"strconv"
)

const (
	metaFieldID       = "id"
	metaFieldName     = "name"
	metaFieldKind     = "kind"
	metaFieldAlias    = "alias"
	metaFieldState    = "state"
	metaFieldRoutes   = "routes"
	metaFieldEvents   = "events"
	metaFieldEndpoint = "endpoint"
)

type registrar struct {
	registry *Registry
}

func newRegistrar(registry *Registry) *registrar {
	return &registrar{registry: registry}
}

// 注册服务
func (r *registrar) register(ctx context.Context, ins *registry.ServiceInstance) error {
	host, port, err := r.parseHostPort(ins.Endpoint)
	if err != nil {
		return err
	}

	routes, err := json.Marshal(ins.Routes)
	if err != nil {
		return err
	}

	events, err := json.Marshal(ins.Events)
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
		Metadata: map[string]string{
			metaFieldID:       ins.ID,
			metaFieldName:     ins.Name,
			metaFieldKind:     ins.Kind,
			metaFieldAlias:    ins.Alias,
			metaFieldState:    ins.State,
			metaFieldRoutes:   string(routes),
			metaFieldEvents:   string(events),
			metaFieldEndpoint: ins.Endpoint,
		},
	}

	ok, err := r.registry.opts.client.RegisterInstance(param)
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("service instance register fail")
	}

	return nil
}

// 解注册服务
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
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
		return errors.New("service instance deregister fail")
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
