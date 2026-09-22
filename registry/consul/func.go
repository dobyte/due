package consul

import (
	"fmt"

	"github.com/dobyte/due/v2/registry"
)

// 构建实例ID
// 拼接 Name 与 ID，避免 ID 相同但服务名不同的实例相互覆盖
func makeInsID(ins *registry.ServiceInstance) string {
	return fmt.Sprintf("%s-%s", ins.Name, ins.ID)
}
