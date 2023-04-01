package direct

import (
	"google.golang.org/grpc/resolver"
)

type Resolver struct {
}

func newResolver() *Resolver {
	return &Resolver{}
}

func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {}

func (r *Resolver) Close() {}
