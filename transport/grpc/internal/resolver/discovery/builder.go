package discovery

import (
	"sync"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
)

const Scheme = "discovery"

type Builder struct {
	rw        sync.RWMutex
	states    map[string]resolver.State
	resolvers sync.Map
}

var _ resolver.Builder = &Builder{}

func NewBuilder() *Builder {
	return &Builder{states: make(map[string]resolver.State)}
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	b.rw.RLock()
	state := b.states[target.URL.Host]
	b.rw.RUnlock()

	r := &Resolver{builder: b, target: target, cc: cc}
	r.updateState(state)
	b.resolvers.Store(target.URL.Host, r)

	return r, nil
}

func (b *Builder) Scheme() string {
	return Scheme
}

func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	states := make(map[string]resolver.State, len(instances))
	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		for _, service := range instance.Services {
			if state, ok := states[service]; ok {
				state.Addresses = append(state.Addresses, resolver.Address{Addr: ep.Address(), ServerName: service})
			} else {
				states[service] = resolver.State{Addresses: []resolver.Address{{Addr: ep.Address(), ServerName: service}}}
			}
		}
	}

	b.rw.Lock()
	b.states = states
	b.rw.Unlock()

	b.resolvers.Range(func(key, value any) bool {
		r := value.(*Resolver)
		r.updateState(states[r.target.URL.Host])
		return true
	})
}
