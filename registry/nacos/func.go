package nacos

import (
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
)

// 解析服务实例列表
func parseInstances(instances []model.Instance) ([]*registry.ServiceInstance, error) {
	services := make([]*registry.ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		if !instance.Healthy || !instance.Enable {
			continue
		}

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
			if err := json.Unmarshal([]byte(v), &ins.Routes); err != nil {
				return nil, err
			}
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

		services = append(services, ins)
	}

	return services, nil
}
