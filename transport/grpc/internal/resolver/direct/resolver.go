package direct

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
)

type Resolver struct {
	builder *Builder
	target  resolver.Target
	cc      resolver.ClientConn
}

func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {
	// ignore
}

func (r *Resolver) Close() {
	log.Warnf("direct resolver close")
	// 从构建器中移除自身，释放引用
	r.builder.resolvers.Delete(r.target.URL.Host)
}

func (r *Resolver) updateState(state resolver.State) {
	if err := r.cc.UpdateState(state); err != nil {
		r.cc.ReportError(err)

		if !(len(state.Addresses) == 0 && errors.Is(err, balancer.ErrBadResolverState)) {
			log.Warnf("update client conn state failed: %v", err)
		}
	}
}
