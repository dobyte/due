package discovery

import (
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
	"sync"
	"time"
)

type Resolver struct {
	builder     *Builder
	serviceName string
	filter      cli.ServiceDiscoveryFilter
	prw         sync.RWMutex
	pairs       []*cli.KVPair
	cmu         sync.RWMutex
	chans       []chan []*cli.KVPair
}

func newResolver(builder *Builder, serviceName string) *Resolver {
	return &Resolver{
		builder:     builder,
		serviceName: serviceName,
	}
}

// GetServices returns the servers
func (r *Resolver) GetServices() []*cli.KVPair {
	r.prw.RLock()
	defer r.prw.RUnlock()

	return r.pairs
}

// WatchService returns a nil chan.
func (r *Resolver) WatchService() chan []*cli.KVPair {
	r.cmu.Lock()
	defer r.cmu.Unlock()

	ch := make(chan []*cli.KVPair, 10)
	r.chans = append(r.chans, ch)
	return ch
}

// RemoveWatcher remove a non-nil chan.
func (r *Resolver) RemoveWatcher(ch chan []*cli.KVPair) {
	r.cmu.Lock()
	defer r.cmu.Unlock()

	chans := make([]chan []*cli.KVPair, 0, len(r.chans))
	for _, c := range r.chans {
		if c == ch {
			close(c)
		} else {
			chans = append(chans, c)
		}
	}
	r.chans = chans
}

// Clone clone a new resolver
func (r *Resolver) Clone(servicePath string) (cli.ServiceDiscovery, error) {
	return r, nil
}

func (r *Resolver) SetFilter(filter cli.ServiceDiscoveryFilter) {
	r.filter = filter
}

func (r *Resolver) Close() {
	r.builder.removeResolver(r.serviceName)

	r.cmu.RLock()
	for _, c := range r.chans {
		close(c)
	}
	r.cmu.RUnlock()
}

func (r *Resolver) updateInstances(instances []*registry.ServiceInstance) {
	pairs := make([]*cli.KVPair, 0, len(instances))

	for _, instance := range instances {
		exists := false

		for _, service := range instance.Services {
			if service == r.serviceName {
				exists = true
				break
			}
		}

		if !exists {
			continue
		}

		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		pair := &cli.KVPair{Key: "tcp@" + ep.Address()}
		if r.filter != nil && !r.filter(pair) {
			continue
		}

		pairs = append(pairs, pair)
	}

	r.prw.Lock()
	r.pairs = pairs
	r.prw.Unlock()

	r.cmu.RLock()
	for _, ch := range r.chans {
		go func(ch chan []*cli.KVPair) {
			defer func() { recover() }()

			select {
			case ch <- pairs:
			case <-time.After(time.Minute):
				log.Warn("chan is full and new change has been dropped")
			}
		}(ch)
	}
	r.cmu.RUnlock()
}
