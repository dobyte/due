package client

import (
	"context"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver"
	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver/direct"
	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver/discovery"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/def"
	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
	proto "github.com/smallnest/rpcx/protocol"
	"golang.org/x/sync/singleflight"
)

const defaultPoolSize = 10

const defaultTimeout = 10 * time.Second

// Builder is the client connection pool builder that creates rpcx connection pools and manages
// their lifecycle.
type Builder struct {
	ctx      context.Context
	cancel   context.CancelFunc
	err      error
	opts     *Options
	dialOpts cli.Option
	builders map[string]resolver.Builder
	sfg      singleflight.Group
	pools    sync.Map
	watcher  registry.Watcher
	closed   atomic.Bool
}

// Options holds the client options.
type Options struct {
	PoolSize   int
	CAFile     string
	ServerName string
	Dispatch   cluster.Dispatch
	Discovery  registry.Discovery
	FailMode   cli.FailMode
}

// NewBuilder returns a new client connection pool builder.
//
// It registers the direct and service discovery resolvers, and initializes the TLS transport
// configuration as needed.
func NewBuilder(opts *Options) *Builder {
	b := &Builder{}
	b.opts = opts
	b.builders = make(map[string]resolver.Builder)
	b.dialOpts = cli.DefaultOption
	b.dialOpts.CompressType = proto.Gzip
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.RegisterBuilder(direct.NewBuilder())
	if opts.Discovery != nil {
		b.RegisterBuilder(discovery.NewBuilder())
	}

	if opts.CAFile != "" && opts.ServerName != "" {
		b.dialOpts.TLSConfig, b.err = tls.MakeTCPClientTLSConfig(opts.CAFile, opts.ServerName)
	} else if opts.CAFile != "" || opts.ServerName != "" {
		log.Warn("rpcx client use insecure credentials")
	}

	if b.err == nil {
		if err := b.init(); err != nil {
			b.cancel()
			b.err = err
		}
	}

	return b
}

// RegisterBuilder registers a resolver builder.
func (b *Builder) RegisterBuilder(builder resolver.Builder) {
	b.builders[builder.Scheme()] = builder
}

// init initializes service discovery by loading the initial instances and starting the instance
// change watcher.
func (b *Builder) init() error {
	if b.opts.Discovery == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(b.ctx, defaultTimeout)
	watcher, err := b.opts.Discovery.Watch(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(b.ctx, defaultTimeout)
	instances, err := b.opts.Discovery.Services(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		_ = watcher.Stop()

		return err
	}

	b.watcher = watcher
	b.updateInstances(instances)

	go b.watch()

	return nil
}

// watch watches service instance changes and synchronizes them to the resolver builders.
func (b *Builder) watch() {
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		instances, err := b.watcher.Next()
		if err != nil {
			if errors.Is(err, errors.ErrWatcherStopped) {
				// The watcher has stopped; exit the loop to avoid spinning.
				return
			}
			// For other errors, back off briefly and retry to avoid a busy loop.
			time.Sleep(time.Second)
			continue
		}

		b.updateInstances(instances)
	}
}

// updateInstances updates the service instance state and dispatches it to the resolver builders.
func (b *Builder) updateInstances(instances []*registry.ServiceInstance) {
	for _, builder := range b.builders {
		builder.UpdateStates(instances)
	}
}

// Build builds a client.
//
// A connection pool for the same target is cached and reused, and singleflight prevents concurrent
// duplicate pool creation.
func (b *Builder) Build(target string) (*cli.OneClient, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.closed.Load() {
		return nil, errors.ErrClientClosed
	}

	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	val, ok := b.pools.Load(target)
	if ok {
		return val.(*cli.OneClientPool).Get(), nil
	}

	val, err, _ = b.sfg.Do(target, func() (any, error) {
		if b.closed.Load() {
			return nil, errors.ErrClientClosed
		}

		builder, ok := b.builders[u.Scheme]
		if !ok {
			return nil, errors.ErrMissingResolver
		}

		dis, err := builder.Build(u)
		if err != nil {
			return nil, err
		}

		size := b.opts.PoolSize
		if size <= 0 {
			size = defaultPoolSize
		}

		var selectMode cli.SelectMode
		switch b.opts.Dispatch {
		case def.Random:
			selectMode = cli.RandomSelect
		case def.WeightedRoundRobin:
			selectMode = cli.WeightedRoundRobin
		case def.ConsistentHash:
			selectMode = cli.ConsistentHash
		default:
			selectMode = cli.RoundRobin
		}

		pool := cli.NewOneClientPool(size, b.opts.FailMode, selectMode, dis, b.dialOpts)

		b.pools.Store(target, pool)

		// Prevent storing an already-closed connection pool when Close and Build run concurrently.
		if b.closed.Load() {
			pool.Close()
			b.pools.Delete(target)
			return nil, errors.ErrClientClosed
		}

		return pool, nil
	})
	if err != nil {
		return nil, err
	}

	return val.(*cli.OneClientPool).Get(), nil
}

// Close closes the builder and releases all connection pools and watch resources. It is idempotent.
func (b *Builder) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}

	var firstErr error

	// Close all connection pools.
	b.pools.Range(func(_, value any) bool {
		value.(*cli.OneClientPool).Close()
		return true
	})
	b.pools.Clear()

	// Notify the watch goroutine to exit.
	if b.cancel != nil {
		b.cancel()
	}

	// Stop the service discovery watcher and unblock Next.
	if b.watcher != nil {
		if err := b.watcher.Stop(); err != nil {
			firstErr = err
		}
	}

	// Close the resolver builders and release the watch resources.
	for _, builder := range b.builders {
		if err := builder.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
