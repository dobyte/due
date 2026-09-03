package etcd

import (
	"fmt"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/registry"
)

// 构建服务实例ID
// 实例ID由服务类型、服务名称与服务实体ID拼接而成
// @param ins *registry.ServiceInstance 服务实例
// @return @1 string 服务实例ID
func makeInsID(ins *registry.ServiceInstance) string {
	return fmt.Sprintf("%s-%s-%s", ins.Kind, ins.Name, ins.ID)
}

func marshal(ins *registry.ServiceInstance) (string, error) {
	buf, err := json.Marshal(ins)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// unmarshal 反序列化服务实例
// @param data []byte 待反序列化的服务实例数据
// @return @1 *registry.ServiceInstance 反序列化后的服务实例
// @return @2 error 反序列化失败时返回的错误
func unmarshal(data []byte) (*registry.ServiceInstance, error) {
	ins := &registry.ServiceInstance{}
	if err := json.Unmarshal(data, ins); err != nil {
		return nil, err
	}
	return ins, nil
}

func buildPrefixKey(namespace, serviceName string) string {
	return fmt.Sprintf("/%s/%s/", namespace, serviceName)
}
