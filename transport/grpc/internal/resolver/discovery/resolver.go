package discovery

import (
	"context"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"google.golang.org/grpc/resolver"
	"time"
)

type Resolver struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cc      resolver.ClientConn
	watcher registry.Watcher
	timeout time.Duration
}

func newResolver(dis registry.Discovery, servicePath string, cc resolver.ClientConn) (*Resolver, error) {
	r := &Resolver{}
	r.cc = cc
	r.timeout = 10 * time.Second
	r.ctx, r.cancel = context.WithCancel(context.Background())

	if err := r.init(dis, servicePath); err != nil {
		return nil, err
	}

	go r.watch()

	return r, nil
}

func (r *Resolver) init(dis registry.Discovery, servicePath string) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
	watcher, err := dis.Watch(ctx, servicePath)
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(ctx, r.timeout)
	services, err := dis.Services(ctx, servicePath)
	cancel()
	if err != nil {
		return err
	}

	r.watcher = watcher
	r.updateServices(services)

	return nil
}

func (r *Resolver) watch() {
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// exec watch
		}
		services, err := r.watcher.Next()
		if err != nil {
			continue
		}

		r.updateServices(services)
	}
}

func (r *Resolver) updateServices(services []*registry.ServiceInstance) {
	state := resolver.State{Addresses: make([]resolver.Address, 0, len(services))}
	for _, service := range services {
		ep, err := endpoint.ParseEndpoint(service.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		state.Addresses = append(state.Addresses, resolver.Address{
			Addr:       ep.Address(),
			ServerName: service.Alias,
		})
	}

	err := r.cc.UpdateState(state)
	if err != nil {
		log.Errorf("update client conn state failed: %v", err)
	}
}

func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {

}

// Close closes the resolver.
func (r *Resolver) Close() {
	r.cancel()
	err := r.watcher.Stop()
	if err != nil {
		log.Errorf("service watcher stop failed: %v", err)
	}
}
