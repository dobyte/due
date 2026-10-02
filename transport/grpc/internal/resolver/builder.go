package resolver

import (
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
)

// Builder is the resolver builder interface.
//
// It extends the standard gRPC resolver builder with the ability to update the state of service
// instances.
type Builder interface {
	resolver.Builder
	// UpdateStates updates the resolver state.
	UpdateStates(instances []*registry.ServiceInstance)
}
