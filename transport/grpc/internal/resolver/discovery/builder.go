package discovery

import (
	"context"
	"github.com/dobyte/due/registry"
	"google.golang.org/grpc/resolver"
	"time"
)

const scheme = "discovery"

type Builder struct {
	dis registry.Discovery
}

var _ resolver.Builder = &Builder{}

func NewBuilder(dis registry.Discovery) *Builder {
	return &Builder{dis: dis}
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	watcher, err := b.dis.Watch(ctx, target.URL.Host)
	cancel()
	if err != nil {
		return nil, err
	}

	r := newResolver(watcher)

	go r.watch(cc)

	return r, nil
}

func (b *Builder) Scheme() string {
	return scheme
}
