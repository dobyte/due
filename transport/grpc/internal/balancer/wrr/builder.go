package wrr

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

// Name is the load balancer name.
const Name = "wrr"

var _ balancer.Builder = &Builder{}

// init registers the weighted round-robin load balancer with the global gRPC builder.
func init() {
	balancer.Register(&Builder{})
}

// Builder is the builder of the weighted round-robin load balancer.
type Builder struct{}

// Build returns a new load balancer instance.
func (b *Builder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	return &Balancer{
		cc:       cc,
		opts:     opts,
		subConns: make(map[balancer.SubConn]resolver.Address),
		scStates: make(map[balancer.SubConn]connectivity.State),
	}
}

// Name returns the load balancer name.
func (b *Builder) Name() string {
	return Name
}
