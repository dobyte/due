package discovery

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
	"sync"
	"time"
)

const scheme = "discovery"

const defaultTimeout = 10 * time.Second

type Builder struct {
	dis       registry.Discovery
	ctx       context.Context
	cancel    context.CancelFunc
	watcher   registry.Watcher
	rw        sync.RWMutex
	services  []*registry.ServiceInstance
	resolvers map[string]*Resolver
}

var _ resolver.Builder = &Builder{}

func NewBuilder(dis registry.Discovery) *Builder {
	b := &Builder{}
	b.dis = dis
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.resolvers = make(map[string]*Resolver)

	if err := b.init(); err != nil {
		log.Fatalf("init client builder failed: %v", err)
	}

	go b.watch()

	return b
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := newResolver(b, target.URL.Host, cc)

	b.rw.Lock()
	services := b.services
	b.resolvers[target.URL.Host] = r
	b.rw.Unlock()

	r.updateServices(services)

	return r, nil
}

func (b *Builder) Scheme() string {
	return scheme
}

func (b *Builder) init() error {
	ctx, cancel := context.WithTimeout(b.ctx, defaultTimeout)
	services, err := b.dis.Services(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(b.ctx, defaultTimeout)
	watcher, err := b.dis.Watch(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	b.watcher = watcher
	b.updateServices(services)

	return nil
}

func (b *Builder) watch() {
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
			// exec watch
		}
		services, err := b.watcher.Next()
		if err != nil {
			continue
		}

		b.updateServices(services)
	}
}

func (b *Builder) updateServices(services []*registry.ServiceInstance) {
	b.rw.Lock()
	defer b.rw.Unlock()

	b.services = services

	for _, r := range b.resolvers {
		r.updateServices(services)
	}
}

func (b *Builder) removeResolver(servicePath string) {
	b.rw.Lock()
	delete(b.resolvers, servicePath)
	b.rw.Unlock()
}
