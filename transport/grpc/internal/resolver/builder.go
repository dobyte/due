package resolver

import (
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
)

// Builder 解析器构建器接口
// 在标准 gRPC 解析器构建器基础上扩展了服务实例状态更新能力
type Builder interface {
	resolver.Builder
	// UpdateStates 更新解析器的状态
	UpdateStates(instances []*registry.ServiceInstance)
}
