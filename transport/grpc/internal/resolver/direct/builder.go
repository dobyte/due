package direct

import (
	"github.com/dobyte/due/registry"
	"google.golang.org/grpc/resolver"
)

const scheme = "direct"

type Builder struct {
	dis registry.Discovery
}

var _ resolver.Builder = &Builder{}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	state := resolver.State{Addresses: make([]resolver.Address, 0, 1)}
	state.Addresses = append(state.Addresses, resolver.Address{
		Addr: target.URL.Host,
	})

	err := cc.UpdateState(state)
	if err != nil {
		return nil, err
	}

	return newResolver(), nil
}

func (b *Builder) Scheme() string {
	return scheme
}
