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
	instances []*registry.ServiceInstance
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

	return b
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	b.rw.RLock()
	r, ok := b.resolvers[target.URL.Host]
	b.rw.RUnlock()

	if ok {
		return r, nil
	}

	b.rw.Lock()
	defer b.rw.Unlock()

	if r, ok = b.resolvers[target.URL.Host]; ok {
		return r, nil
	}

	r = newResolver(b, target.URL.Host, cc)
	r.updateInstances(b.instances)

	b.resolvers[target.URL.Host] = r

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
	b.updateInstances(services)

	go b.watch()

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

		b.updateInstances(services)
	}
}

func (b *Builder) updateInstances(instances []*registry.ServiceInstance) {
	b.rw.Lock()
	defer b.rw.Unlock()

	b.instances = instances

	for _, r := range b.resolvers {
		r.updateInstances(instances)
	}
}

func (b *Builder) removeResolver(servicePath string) {
	b.rw.Lock()
	delete(b.resolvers, servicePath)
	b.rw.Unlock()
}
