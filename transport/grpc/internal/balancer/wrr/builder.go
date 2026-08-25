package wrr

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

// Name 负载均衡器名称
const Name = "wrr"

var _ balancer.Builder = &Builder{}

// init 注册加权轮询负载均衡器到 gRPC 全局构建器
func init() {
	balancer.Register(&Builder{})
}

// Builder 加权轮询负载均衡器构建器
type Builder struct{}

// Build 构建负载均衡器实例
// @param cc balancer.ClientConn 客户端连接
// @param opts balancer.BuildOptions 构建选项
// @return @1 balancer.Balancer 负载均衡器实例
func (b *Builder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	return &Balancer{
		cc:       cc,
		opts:     opts,
		subConns: make(map[balancer.SubConn]resolver.Address),
		scStates: make(map[balancer.SubConn]connectivity.State),
	}
}

// Name 获取负载均衡器名称
// @return @1 string 名称
func (b *Builder) Name() string {
	return Name
}
