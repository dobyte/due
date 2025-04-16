package discovery

import (
	"github.com/dobyte/due/v2/log"
	cli "github.com/smallnest/rpcx/client"
	"sync"
	"time"
)

type Resolver struct {
	builder *Builder
	name    string
	filter  cli.ServiceDiscoveryFilter
	prw     sync.RWMutex
	pairs   []*cli.KVPair
	crw     sync.RWMutex
	chans   []chan []*cli.KVPair
}

func newResolver(name string, builder *Builder) *Resolver {
	return &Resolver{
		name:    name,
		builder: builder,
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
	ch := make(chan []*cli.KVPair, 10)

	r.crw.Lock()
	r.chans = append(r.chans, ch)
	r.crw.Unlock()

	return ch
}

// RemoveWatcher remove a non-nil chan.
func (r *Resolver) RemoveWatcher(ch chan []*cli.KVPair) {
	r.crw.Lock()
	defer r.crw.Unlock()

	i := -1

	for _, c := range r.chans {
		if c == ch {
			close(c)
		} else {
			i++
			r.chans[i] = c
		}
	}

	r.chans = r.chans[:i+1]
}

// Clone clone a new resolver
func (r *Resolver) Clone(servicePath string) (cli.ServiceDiscovery, error) {
	return r, nil
}

func (r *Resolver) SetFilter(filter cli.ServiceDiscoveryFilter) {
	r.filter = filter
}

func (r *Resolver) Close() {
	r.builder.removeResolver(r)

	r.crw.RLock()
	for _, c := range r.chans {
		close(c)
	}
	r.crw.RUnlock()
}

func (r *Resolver) updateState(list []*cli.KVPair) {
	var pairs []*cli.KVPair

	if r.filter != nil {
		pairs = make([]*cli.KVPair, 0, len(list))
		for _, pair := range list {
			if r.filter(pair) {
				pairs = append(pairs, pair)
			}
		}
	} else {
		pairs = list
	}

	r.prw.Lock()
	r.pairs = pairs
	r.prw.Unlock()

	r.crw.RLock()
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
	r.crw.RUnlock()
}
