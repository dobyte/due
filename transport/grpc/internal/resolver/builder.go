package resolver

import (
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
)

type Builder interface {
	resolver.Builder
	// UpdateStates 更新解析器的状态
	UpdateStates(instances []*registry.ServiceInstance)
}
