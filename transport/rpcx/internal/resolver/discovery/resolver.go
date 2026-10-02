package discovery

import (
	"sync"
	"time"

	"github.com/dobyte/due/v2/log"
	cli "github.com/smallnest/rpcx/client"
)

// Resolver is the service discovery mode service discovery instance.
//
// It maintains the service address list and lets subscribers receive address change notifications.
type Resolver struct {
	builder *Builder
	name    string
	filter  cli.ServiceDiscoveryFilter
	prw     sync.RWMutex
	pairs   []*cli.KVPair
	crw     sync.RWMutex
	chans   []chan []*cli.KVPair
	closed  bool
}

// newResolver returns a new service discovery instance for service discovery mode.
func newResolver(name string, builder *Builder) *Resolver {
	return &Resolver{
		name:    name,
		builder: builder,
	}
}

// GetServices returns the service address list.
func (r *Resolver) GetServices() []*cli.KVPair {
	r.prw.RLock()
	defer r.prw.RUnlock()

	return r.pairs
}

// WatchService watches service address changes.
//
// It returns a buffered change notification channel, or an already closed channel when the resolver
// has been closed.
func (r *Resolver) WatchService() chan []*cli.KVPair {
	ch := make(chan []*cli.KVPair, 10)

	r.crw.Lock()
	if r.closed {
		r.crw.Unlock()
		close(ch)
		return ch
	}
	r.chans = append(r.chans, ch)
	r.crw.Unlock()

	return ch
}

// RemoveWatcher removes a watch channel.
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

// Clone clones the service discovery instance, reusing the current instance directly.
func (r *Resolver) Clone(servicePath string) (cli.ServiceDiscovery, error) {
	return r, nil
}

// SetFilter sets the service filter function.
func (r *Resolver) SetFilter(filter cli.ServiceDiscoveryFilter) {
	r.filter = filter
}

// Close closes the service discovery instance.
//
// It removes itself from the builder and closes all watch channels.
func (r *Resolver) Close() {
	r.builder.removeResolver(r)

	r.crw.Lock()
	if r.closed {
		r.crw.Unlock()
		return
	}
	r.closed = true
	for _, c := range r.chans {
		close(c)
	}
	r.chans = nil
	r.crw.Unlock()
}

// updateState updates the service address state and broadcasts the change.
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
	defer r.crw.RUnlock()

	if r.closed {
		return
	}

	for _, ch := range r.chans {
		select {
		case ch <- pairs:
			// Fast path: the consumer reads in time, no goroutine is spawned.
		default:
			// Slow path: the channel is full; wait up to 1 minute before dropping.
			go func(ch chan []*cli.KVPair) {
				defer func() { recover() }()

				timer := time.NewTimer(time.Minute)
				defer timer.Stop()

				select {
				case ch <- pairs:
				case <-timer.C:
					log.Warn("chan is full and new change has been dropped")
				}
			}(ch)
		}
	}
}
