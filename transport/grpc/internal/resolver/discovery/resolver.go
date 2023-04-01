package discovery

import (
	"context"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"google.golang.org/grpc/resolver"
)

type Resolver struct {
	ctx     context.Context
	cancel  context.CancelFunc
	watcher registry.Watcher
}

func newResolver(watcher registry.Watcher) *Resolver {
	r := &Resolver{}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.watcher = watcher
	return r
}

func (r *Resolver) watch(cc resolver.ClientConn) {
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

		err = cc.UpdateState(state)
		if err != nil {
			log.Errorf("update client conn state failed: %v", err)
		}
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
