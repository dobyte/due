package discovery

import (
	"github.com/dobyte/due/registry"
	"google.golang.org/grpc/resolver"
)

const scheme = "discovery"

type Builder struct {
	dis registry.Discovery
}

var _ resolver.Builder = &Builder{}

func NewBuilder(dis registry.Discovery) *Builder {
	return &Builder{dis: dis}
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	return newResolver(b.dis, target.URL.Host, cc)
}

func (b *Builder) Scheme() string {
	return scheme
}
