package nacos

import (
	"net"
	"net/url"
	"strings"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
)

// nacosMetadataLimit is the default total length limit of instance metadata keys and values
// enforced by Nacos 2.3.0+. Older versions do not enforce it.
const nacosMetadataLimit = 1024

var errAllInstancesParseFailed = errors.New("all instances parse failed")

// parseInstances parses a service instance list.
//
// Instances with invalid metadata are logged and skipped. An error is returned only when every
// healthy instance fails to parse, so that callers keep the previous snapshot instead of clearing
// the routing table.
func parseInstances(instances []model.Instance) ([]*registry.ServiceInstance, error) {
	services := make([]*registry.ServiceInstance, 0, len(instances))

	var total, failed int

	for _, instance := range instances {
		if !instance.Healthy || !instance.Enable {
			continue
		}

		total++

		ins, err := parseInstance(instance)
		if err != nil {
			failed++

			log.Warnf("instance parse failed, insID: %s kind: %s name: %s endpoint: %s err: %v",
				instance.Metadata[metaFieldID], instance.Metadata[metaFieldKind],
				instance.Metadata[metaFieldName], instance.Metadata[metaFieldEndpoint], err)
			continue
		}

		services = append(services, ins)
	}

	if total > 0 && failed == total {
		return nil, errAllInstancesParseFailed
	}

	return services, nil
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

// metadataSize returns the total length of all metadata keys and values.
func metadataSize(metadata map[string]string) int {
	size := 0

	for key, value := range metadata {
		size += len(key) + len(value)
	}

	return size
}
