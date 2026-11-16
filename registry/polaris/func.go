package polaris

import (
	stdnet "net"
	"net/url"
	"strconv"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// parseEndpoint parses a service instance endpoint.
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

// parseInstances parses a list of service instances.
//
// Instances with invalid metadata are logged and skipped. An error is returned only when every
// healthy instance fails to parse, so that callers keep the previous snapshot instead of clearing
// the routing table.
func parseInstances(instances []model.Instance) ([]*registry.ServiceInstance, error) {
	var (
		total    int
		failed   int
		services = make([]*registry.ServiceInstance, 0, len(instances))
	)

	for _, instance := range instances {
		if !instance.IsHealthy() || instance.IsIsolated() {
			continue
		}

		total++

		ins, err := parseInstance(instance)
		if err != nil {
			failed++

			metadata := instance.GetMetadata()
			log.Warnf("instance parse failed, insID: %s kind: %s name: %s endpoint: %s err: %v",
				metadata[metaFieldID], metadata[metaFieldKind],
				metadata[metaFieldName], metadata[metaFieldEndpoint], err)
			continue
		}

		services = append(services, ins)
	}

	if total > 0 && failed == total {
		return nil, errors.ErrAllInstancesParseFailed
	}

	return services, nil
}

// parseInstance parses one service instance.
func parseInstance(instance model.Instance) (*registry.ServiceInstance, error) {
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
		routes, err := unmarshalRoutes(v)
		if err != nil {
			return nil, err
		}

		ins.Routes = routes
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

	return ins, nil
}
