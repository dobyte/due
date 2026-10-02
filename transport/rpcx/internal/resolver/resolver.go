package resolver

import (
	"net/url"

	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
)

// Builder is the resolver builder interface.
//
// It builds a service discovery instance for the target address and receives service instance state
// updates.
type Builder interface {
	// Build builds a service discovery instance.
	Build(target *url.URL) (cli.ServiceDiscovery, error)
	// Scheme returns the resolver scheme.
	Scheme() string
	// UpdateStates updates the state of service instances.
	UpdateStates(instances []*registry.ServiceInstance)
	// Close closes the builder and releases the watch resources.
	Close() error
}
