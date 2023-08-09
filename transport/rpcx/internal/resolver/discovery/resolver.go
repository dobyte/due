package discovery

import (
	"context"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
	"sync"
	"time"
)

type Resolver struct {
	ctx         context.Context
	cancel      context.CancelFunc
	dis         registry.Discovery
	servicePath string
	watcher     registry.Watcher
	filter      cli.ServiceDiscoveryFilter

	prw   sync.RWMutex
	pairs []*cli.KVPair

	cmu   sync.RWMutex
	chans []chan []*cli.KVPair
}

func newResolver(dis registry.Discovery, servicePath string) (*Resolver, error) {
	r := &Resolver{}
	r.dis = dis
	r.servicePath = servicePath
	r.ctx, r.cancel = context.WithCancel(context.Background())

	if err := r.init(); err != nil {
		return nil, err
	}

	go r.watch()

	return r, nil
}

func (r *Resolver) init() error {
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	watcher, err := r.dis.Watch(ctx, r.servicePath)
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	services, err := r.dis.Services(ctx, r.servicePath)
	cancel()
	if err != nil {
		return err
	}

	r.watcher = watcher
	r.pairs = make([]*cli.KVPair, 0, len(services))

	for _, service := range services {
		ep, err := endpoint.ParseEndpoint(service.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		pair := &cli.KVPair{Key: "tcp@" + ep.Address()}
		if r.filter != nil && !r.filter(pair) {
			continue
		}

		r.pairs = append(r.pairs, pair)
	}

	return nil
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
			continue
		}
		chans = append(chans, c)
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
	r.cancel()

	if err := r.watcher.Stop(); err != nil {
		log.Errorf("dispatcher watcher stop failed: %v", err)
	}
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

		pairs := make([]*cli.KVPair, 0, len(services))
		for _, service := range services {
			ep, err := endpoint.ParseEndpoint(service.Endpoint)
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
}
