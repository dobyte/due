package random

import (
	"math/rand"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

const Name = "random"

var _ balancer.Builder = &Builder{}

func init() {
	balancer.Register(&Builder{})
}

type Builder struct{}

func (b *Builder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	return &Balancer{
		cc:       cc,
		opts:     opts,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
		subConns: make(map[balancer.SubConn]resolver.Address),
		scStates: make(map[balancer.SubConn]connectivity.State),
	}
}

func (b *Builder) Name() string {
	return Name
}
