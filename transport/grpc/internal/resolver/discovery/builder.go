package discovery

import (
	"sync"

	"github.com/dobyte/due/transport/grpc/v2/internal/balancer/wrr"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
)

const scheme = "discovery"

type Builder struct {
	rw        sync.RWMutex
	states    map[string]*resolver.State
	resolvers sync.Map
}

var _ resolver.Builder = &Builder{}

func NewBuilder() *Builder {
	return &Builder{states: make(map[string]*resolver.State)}
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	b.rw.RLock()
	state := b.states[target.URL.Host]
	b.rw.RUnlock()

	r := &Resolver{builder: b, target: target, cc: cc}

	if state != nil {
		r.updateState(*state)
	} else {
		r.updateState(resolver.State{})
	}

	b.resolvers.Store(target.URL.Host, r)

	return r, nil
}

func (b *Builder) Scheme() string {
	return scheme
}

func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	var (
		states     map[string]*resolver.State
		workStates = make(map[string]*resolver.State, len(instances))
		busyStates = make(map[string]*resolver.State, len(instances))
		hangStates = make(map[string]*resolver.State, len(instances))
	)

	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		switch instance.State {
		case cluster.Work.String():
			states = workStates
		case cluster.Busy.String():
			states = busyStates
		case cluster.Hang.String():
			states = hangStates
		default:
			continue
		}

		for _, service := range instance.Services {
			addr := resolver.Address{
				Addr:       ep.Address(),
				ServerName: service,
				Attributes: attributes.New(wrr.WeightAttrKey, uint32(max(1, instance.Weight))),
			}

			if state, ok := states[service]; ok {
				state.Addresses = append(state.Addresses, addr)
			} else {
				states[service] = &resolver.State{Addresses: []resolver.Address{addr}}
			}
		}
	}

	switch {
	case len(workStates) > 0:
		states = workStates
	case len(busyStates) > 0:
		states = busyStates
	case len(hangStates) > 0:
		states = hangStates
	default:
		states = make(map[string]*resolver.State)
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
