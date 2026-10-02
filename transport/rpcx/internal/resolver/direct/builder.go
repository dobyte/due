package direct

import (
	"net"
	"net/url"
	"sync"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
)

const scheme = "direct"

// Builder is the resolver builder for direct connection mode.
//
// It supports both direct://address and direct://instance_ID forms.
type Builder struct {
	rw        sync.RWMutex
	pairs     map[string][]*cli.KVPair
	resolvers sync.Map
}

// NewBuilder returns a new builder for the direct connection resolver.
func NewBuilder() *Builder {
	return &Builder{}
}

// Scheme returns the resolver scheme.
func (b *Builder) Scheme() string {
	return scheme
}

// Build builds a service discovery instance.
//
// It returns a peer-to-peer discovery instance when the address can be parsed directly as host:port,
// and otherwise looks up the cached address by instance ID.
func (b *Builder) Build(target *url.URL) (cli.ServiceDiscovery, error) {
	if _, _, err := net.SplitHostPort(target.Host); err == nil {
		return cli.NewPeer2PeerDiscovery("tcp@"+target.Host, "")
	}

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
// It aggregates instance endpoints into address pairs keyed by instance ID, and pushes an empty
// state when an instance goes offline.
func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	pairs := make(map[string][]*cli.KVPair, len(instances))
	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		pairs[instance.ID] = append(pairs[instance.ID], &cli.KVPair{Key: "tcp@" + ep.Address()})
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
