package nacos

import (
	"net"
	"net/url"
	"strings"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
)

// parseInstances parses a service instance list.
//
// Instances with invalid metadata are logged and skipped, so that one bad instance does not block
// the discovery updates of the whole service.
func parseInstances(instances []model.Instance) []*registry.ServiceInstance {
	services := make([]*registry.ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		if !instance.Healthy || !instance.Enable {
			continue
		}

		ins, err := parseInstance(instance)
		if err != nil {
			log.Warnf("instance parse failed, insID: %s kind: %s name: %s endpoint: %s err: %v",
				instance.Metadata[metaFieldID], instance.Metadata[metaFieldKind],
				instance.Metadata[metaFieldName], instance.Metadata[metaFieldEndpoint], err)
			continue
		}

		services = append(services, ins)
	}

	return services
}

// parseInstance parses one service instance.
func parseInstance(instance model.Instance) (*registry.ServiceInstance, error) {
	ins := &registry.ServiceInstance{}
	ins.ID = instance.Metadata[metaFieldID]
	ins.Name = instance.Metadata[metaFieldName]
	ins.Kind = instance.Metadata[metaFieldKind]
	ins.Alias = instance.Metadata[metaFieldAlias]
	ins.State = instance.Metadata[metaFieldState]
	ins.Endpoint = instance.Metadata[metaFieldEndpoint]
	ins.Routes = make([]registry.Route, 0)
	ins.Events = make([]int, 0)
	ins.Services = make([]string, 0)
	ins.Weight = xconv.Int(instance.Metadata[metaFieldWeight])
	ins.Metadata = make(map[string]string)

	if v := instance.Metadata[metaFieldRoutes]; v != "" {
		routes, err := unmarshalRoutes(v)
		if err != nil {
			return nil, err
		}

		ins.Routes = routes
	}

	if v := instance.Metadata[metaFieldEvents]; v != "" {
		if err := json.Unmarshal([]byte(v), &ins.Events); err != nil {
			return nil, err
		}
	}

	if v := instance.Metadata[metaFieldServices]; v != "" {
		if err := json.Unmarshal([]byte(v), &ins.Services); err != nil {
			return nil, err
		}
	}

	if v := instance.Metadata[metaFieldMetadata]; v != "" {
		if err := json.Unmarshal([]byte(v), &ins.Metadata); err != nil {
			return nil, err
		}
	}

	return ins, nil
}

// parseServerConfig parses one server address, prepending http when no scheme is present, so that
// the address list accepts the same forms as the config source.
func parseServerConfig(addr string) (constant.ServerConfig, error) {
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}

	raw, err := url.Parse(addr)
	if err != nil {
		return constant.ServerConfig{}, err
	}

	host, port, err := net.SplitHostPort(raw.Host)
	if err != nil {
		return constant.ServerConfig{}, err
	}

	return constant.ServerConfig{
		Scheme:      raw.Scheme,
		ContextPath: raw.Path,
		IpAddr:      host,
		Port:        xconv.Uint64(port),
	}, nil
}
