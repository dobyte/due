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

// Builder is the resolver builder for service discovery mode.
//
// It obtains service instances from the registry and aggregates addresses by service name and
// instance state.
type Builder struct {
	rw        sync.RWMutex
	states    map[string]*resolver.State
	resolvers sync.Map
}

var _ resolver.Builder = &Builder{}

// NewBuilder returns a new builder for the service discovery resolver.
func NewBuilder() *Builder {
	return &Builder{states: make(map[string]*resolver.State)}
}

// Build builds a resolver.
//
// It looks up the addresses for the service name in the cached states and pushes them.
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

// Scheme returns the resolver scheme.
func (b *Builder) Scheme() string {
	return scheme
}

// UpdateStates updates the state of service instances and synchronizes it to each resolver.
//
// It groups instances by state (work/busy/hang), pushes the most available group first, and
// attaches the instance weight to the address attributes for weighted load balancing.
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
				Attributes: attributes.New(wrr.WeightAttrKey, uint32(max(1, instance.Weight))),
			}

			if state, ok := states[service]; ok {
				state.Addresses = append(state.Addresses, addr)
			} else {
				states[service] = &resolver.State{Addresses: []resolver.Address{addr}}
			}
		}
	}

	// Collect all business service names that appear in any of the three tiers.
	services := make(map[string]struct{}, len(workStates)+len(busyStates)+len(hangStates))
	for service := range workStates {
		services[service] = struct{}{}
	}
	for service := range busyStates {
		services[service] = struct{}{}
	}
	for service := range hangStates {
		services[service] = struct{}{}
	}

	// Select the priority independently per service: Work > Busy > Hang.
	states = make(map[string]*resolver.State, len(services))
	for service := range services {
		switch {
		case workStates[service] != nil:
			states[service] = workStates[service]
		case busyStates[service] != nil:
			states[service] = busyStates[service]
		case hangStates[service] != nil:
			states[service] = hangStates[service]
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
