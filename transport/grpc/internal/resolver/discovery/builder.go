package discovery

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
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
	states    map[string]*resolver.State
	resolvers sync.Map
}

var _ resolver.Builder = &Builder{}

func NewBuilder(dis registry.Discovery) *Builder {
	b := &Builder{}
	b.dis = dis
	b.ctx, b.cancel = context.WithCancel(context.Background())

	if err := b.init(); err != nil {
		log.Fatalf("init client builder failed: %v", err)
	}

	return b
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	b.rw.RLock()
	state, ok := b.states[target.URL.Host]
	b.rw.RUnlock()

	if !ok {
		return nil, errors.ErrNotFoundServiceAddress
	}

	if err := cc.UpdateState(*state); err != nil {
		return nil, err
	}

	r := &Resolver{builder: b, target: target, cc: cc}

	b.resolvers.Store(target.URL.Host, r)

	return r, nil
}

func (b *Builder) Scheme() string {
	return scheme
}

func (b *Builder) init() error {
	if b.dis == nil {
		return errors.ErrMissingDiscovery
	}

	ctx, cancel := context.WithTimeout(b.ctx, defaultTimeout)
	instances, err := b.dis.Services(ctx, cluster.Mesh.String())
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
	b.updateInstances(instances)

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
		instances, err := b.watcher.Next()
		if err != nil {
			continue
		}

		b.updateInstances(instances)
	}
}

func (b *Builder) updateInstances(instances []*registry.ServiceInstance) {
	states := make(map[string]*resolver.State, len(instances))
	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		for _, service := range instance.Services {
			if state, ok := states[service]; ok {
				state.Addresses = append(state.Addresses, resolver.Address{Addr: ep.Address(), ServerName: service})
			} else {
				states[service] = &resolver.State{Addresses: []resolver.Address{{Addr: ep.Address(), ServerName: service}}}
			}
		}
	}

	b.rw.Lock()
	b.states = states
	b.rw.Unlock()

	b.resolvers.Range(func(_, value any) bool {
		r := value.(*Resolver)

		if state, ok := states[r.target.URL.Host]; ok {
			r.updateState(*state)
		} else {
			b.removeResolver(r)
		}

		return true
	})
}

func (b *Builder) updateResolver(r *Resolver) {
	b.rw.RLock()
	states := b.states
	b.rw.RUnlock()

	if state, ok := states[r.target.URL.Host]; ok {
		r.updateState(*state)
	} else {
		b.resolvers.Delete(r.target.URL.Host)
	}
}

func (b *Builder) removeResolver(r *Resolver) {
	b.resolvers.Delete(r.target.URL.Host)
}
