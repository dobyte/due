package etcd

import (
	"fmt"
	"github.com/dobyte/due/v2/registry"
)

// 构建实例ID
func makeInsID(ins *registry.ServiceInstance) string {
	return fmt.Sprintf("%s-%s", ins.Kind, ins.ID)
}
