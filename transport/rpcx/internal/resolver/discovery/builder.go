package discovery

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
)

const scheme = "discovery"

// Builder is the resolver builder for service discovery mode.
//
// It obtains service instances from the registry and aggregates addresses by service name and
// instance state.
type Builder struct {
	rw        sync.RWMutex
	pairs     map[string][]*cli.KVPair
	resolvers sync.Map
}

// NewBuilder returns a new builder for the service discovery resolver.
func NewBuilder() *Builder {
	return &Builder{}
}

// Scheme returns the resolver scheme.
func (b *Builder) Scheme() string {
	return scheme
}

// Build builds a service discovery instance.
//
// It looks up the addresses for the service name in the cached state and pushes them.
func (b *Builder) Build(target *url.URL) (cli.ServiceDiscovery, error) {
	b.rw.RLock()
	pairs, ok := b.pairs[target.Host]
	b.rw.RUnlock()

	if !ok {
		return nil, errors.ErrNotFoundServiceAddress
	}

	r := newResolver(target.Host, b)
	r.updateState(pairs)

	b.resolvers.Store(target.Host, r)

	return r, nil
}

// UpdateStates updates the state of service instances and synchronizes it to each service discovery
// instance.
//
// It groups instances by state (work/busy/hang), pushes the most available group first, and
// attaches the instance weight to the address value for weighted load balancing.
func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	var (
		pairs     map[string][]*cli.KVPair
		workPairs = make(map[string][]*cli.KVPair, len(instances))
		busyPairs = make(map[string][]*cli.KVPair, len(instances))
		hangPairs = make(map[string][]*cli.KVPair, len(instances))
	)

	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		switch instance.State {
		case cluster.Work.String():
			pairs = workPairs
		case cluster.Busy.String():
			pairs = busyPairs
		case cluster.Hang.String():
			pairs = hangPairs
		default:
			continue
		}

		for _, service := range instance.Services {
			pairs[service] = append(pairs[service], &cli.KVPair{
				Key:   "tcp@" + ep.Address(),
				Value: fmt.Sprintf("weight=%d", max(1, instance.Weight)),
			})
		}
	}

	// Collect all business service names that appear in any of the three tiers.
	services := make(map[string]struct{}, len(workPairs)+len(busyPairs)+len(hangPairs))
	for service := range workPairs {
		services[service] = struct{}{}
	}
	for service := range busyPairs {
		services[service] = struct{}{}
	}
	for service := range hangPairs {
		services[service] = struct{}{}
	}

	// Select the priority independently per service: Work > Busy > Hang.
	pairs = make(map[string][]*cli.KVPair, len(services))
	for service := range services {
		switch {
		case workPairs[service] != nil:
			pairs[service] = workPairs[service]
		case busyPairs[service] != nil:
			pairs[service] = busyPairs[service]
		case hangPairs[service] != nil:
			pairs[service] = hangPairs[service]
		}
	}

	b.rw.Lock()
	b.pairs = pairs
	b.rw.Unlock()

	b.resolvers.Range(func(_, value any) bool {
		r := value.(*Resolver)
		r.updateState(pairs[r.name])
		return true
	})
}

// removeResolver removes a service discovery instance.
func (b *Builder) removeResolver(r *Resolver) {
	b.resolvers.Delete(r.name)
}

// Close closes the builder and releases all service discovery instances.
func (b *Builder) Close() error {
	b.resolvers.Range(func(_, value any) bool {
		value.(*Resolver).Close()
		return true
	})

	return nil
}
