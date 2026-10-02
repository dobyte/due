package etcd

import (
	"fmt"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/registry"
)

// makeInsID returns the service instance ID built by joining the service name and the instance ID.
func makeInsID(ins *registry.ServiceInstance) string {
	return fmt.Sprintf("%s-%s", ins.Name, ins.ID)
}

func marshal(ins *registry.ServiceInstance) (string, error) {
	buf, err := json.Marshal(ins)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// unmarshal decodes data into a service instance.
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
