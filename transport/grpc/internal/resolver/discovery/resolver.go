package discovery

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
)

// Resolver 服务发现模式解析器
type Resolver struct {
	builder *Builder
	target  resolver.Target
	cc      resolver.ClientConn
}

// ResolveNow 重新解析，服务发现模式忽略该回调
// @param _ resolver.ResolveNowOptions 解析选项
func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {
	// ignore
}

// Close 关闭解析器
// 从构建器中移除自身，释放引用
func (r *Resolver) Close() {
	r.builder.resolvers.Delete(r.target.URL.Host)
}

// updateState 更新解析状态到客户端连接
// @param state resolver.State 解析状态
func (r *Resolver) updateState(state resolver.State) {
	if err := r.cc.UpdateState(state); err != nil {
		r.cc.ReportError(err)

		if !(len(state.Addresses) == 0 && errors.Is(err, balancer.ErrBadResolverState)) {
			log.Warnf("update client conn state failed: %v", err)
		}
	}
}
