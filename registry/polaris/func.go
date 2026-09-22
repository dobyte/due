package polaris

import (
	stdnet "net"
	"net/url"
	"strconv"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// parseEndpoint 解析服务实例端点
func parseEndpoint(endpoint string) (host string, port int, protocol string, err error) {
	raw, err := url.Parse(endpoint)
	if err != nil {
		return "", 0, "", err
	}

	host, p, err := stdnet.SplitHostPort(raw.Host)
	if err != nil {
		return "", 0, "", err
	}

	port, err = strconv.Atoi(p)
	if err != nil {
		return "", 0, "", err
	}

	return host, port, raw.Scheme, nil
}

// parseInstances 解析服务实例列表
func parseInstances(instances []model.Instance) ([]*registry.ServiceInstance, error) {
	services := make([]*registry.ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		if !instance.IsHealthy() || instance.IsIsolated() {
			continue
		}

		metadata := instance.GetMetadata()

		ins := &registry.ServiceInstance{}
		ins.ID = metadata[metaFieldID]
		ins.Name = metadata[metaFieldName]
		ins.Kind = metadata[metaFieldKind]
		ins.Alias = metadata[metaFieldAlias]
		ins.State = metadata[metaFieldState]
		ins.Endpoint = metadata[metaFieldEndpoint]
		ins.Routes = make([]registry.Route, 0)
		ins.Events = make([]int, 0)
		ins.Services = make([]string, 0)
		ins.Weight = xconv.Int(metadata[metaFieldWeight])
		ins.Metadata = make(map[string]string)

		if v := metadata[metaFieldRoutes]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Routes); err != nil {
				return nil, err
			}
		}

		if v := metadata[metaFieldEvents]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Events); err != nil {
				return nil, err
			}
		}

		if v := metadata[metaFieldServices]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Services); err != nil {
				return nil, err
			}
		}

		if v := metadata[metaFieldMetadata]; v != "" {
			if err := json.Unmarshal([]byte(v), &ins.Metadata); err != nil {
				return nil, err
			}
		}

		services = append(services, ins)
	}

	return services, nil
}
