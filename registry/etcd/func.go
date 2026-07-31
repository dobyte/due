package etcd

import (
	"fmt"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/registry"
)

// 指数退避：100ms 起始，每次翻倍，上限 10s
// attempt 为 0-based 的重试次数
func backoff(attempt int) time.Duration {
	d := 100 * time.Millisecond
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= 10*time.Second {
			return 10 * time.Second
		}
	}
	return d
}

// 构建实例ID
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

func unmarshal(data []byte) (*registry.ServiceInstance, error) {
	ins := &registry.ServiceInstance{}
	if err := json.Unmarshal(data, ins); err != nil {
		return nil, err
	}
	return ins, nil
}

func buildPrefixKey(namespace, serviceName string) string {
	return fmt.Sprintf("/%s/%s", namespace, serviceName)
}
