package direct

import (
	"net"
	"sync"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
)

const scheme = "direct"

// Builder is the resolver builder for direct connection mode.
//
// It supports both direct://address and direct://instance_ID forms.
type Builder struct {
	rw        sync.RWMutex
	states    map[string]*resolver.State
	resolvers sync.Map
}

var _ resolver.Builder = &Builder{}

// NewBuilder returns a new builder for the direct connection resolver.
func NewBuilder() *Builder {
	return &Builder{states: make(map[string]*resolver.State)}
}

// Build builds a resolver.
//
// An address is parsed directly as host:port, while an instance ID is looked up in the cached
// states.
func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &Resolver{builder: b, target: target, cc: cc}

	if _, _, err := net.SplitHostPort(target.URL.Host); err == nil {
		r.updateState(resolver.State{Addresses: []resolver.Address{{Addr: target.URL.Host}}})
	} else {
		b.rw.RLock()
		state := b.states[target.URL.Host]
		b.rw.RUnlock()

		if state != nil {
			r.updateState(*state)
		} else {
			r.updateState(resolver.State{})
		}

		b.resolvers.Store(target.URL.Host, r)
	}

	return r, nil
}

// Scheme returns the resolver scheme.
func (b *Builder) Scheme() string {
	return scheme
}

// UpdateStates updates the state of service instances and synchronizes it to each resolver.
//
// It aggregates instance endpoints into address states keyed by instance ID, and pushes an empty
// state when an instance goes offline.
func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	states := make(map[string]*resolver.State, len(instances))
	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		if state, ok := states[instance.ID]; ok {
			state.Addresses = append(state.Addresses, resolver.Address{Addr: ep.Address()})
		} else {
			states[instance.ID] = &resolver.State{Addresses: []resolver.Address{{Addr: ep.Address()}}}
		}
	}

	b.rw.Lock()
	b.states = states
	b.rw.Unlock()

	b.resolvers.Range(func(key, value any) bool {
		r := value.(*Resolver)
		if state, ok := states[r.target.URL.Host]; ok {
			r.updateState(*state)
		} else {
			r.updateState(resolver.State{})
		}
		return true
	})
}
