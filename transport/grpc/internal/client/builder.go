package client

import (
	"context"
	"sync"
	"time"

	iresolver "github.com/dobyte/due/transport/grpc/v2/internal/resolver"
	"github.com/dobyte/due/transport/grpc/v2/internal/resolver/direct"
	"github.com/dobyte/due/transport/grpc/v2/internal/resolver/discovery"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/registry"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

const defaultTimeout = 10 * time.Second

type Options struct {
	CAFile     string
	ServerName string
	Discovery  registry.Discovery
	DialOpts   []grpc.DialOption
}

type Builder struct {
	ctx         context.Context
	cancel      context.CancelFunc
	err         error
	opts        *Options
	dialOpts    []grpc.DialOption
	resolvers   []resolver.Builder
	sfg         singleflight.Group
	connections sync.Map
	watcher     registry.Watcher
}

func NewBuilder(opts *Options) *Builder {
	var (
		err  error
		cred credentials.TransportCredentials
	)

	if opts.CAFile != "" && opts.ServerName != "" {
		if cred, err = credentials.NewClientTLSFromFile(opts.CAFile, opts.ServerName); err != nil {
			return &Builder{err: err}
		}
	} else {
		cred = insecure.NewCredentials()
	}

	resolvers := make([]resolver.Builder, 0, 2)
	resolvers = append(resolvers, direct.NewBuilder())

	if opts.Discovery != nil {
		resolvers = append(resolvers, discovery.NewBuilder())
	}

	b := &Builder{}
	b.opts = opts
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.resolvers = resolvers
	b.dialOpts = make([]grpc.DialOption, 0, len(opts.DialOpts)+2)
	b.dialOpts = append(b.dialOpts, opts.DialOpts...)
	b.dialOpts = append(b.dialOpts, grpc.WithTransportCredentials(cred))
	b.dialOpts = append(b.dialOpts, grpc.WithResolvers(resolvers...))
	b.dialOpts = append(b.dialOpts, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`))

	if err := b.init(); err != nil {
		return &Builder{err: err}
	}

	return b
}

func (b *Builder) init() error {
	if b.opts.Discovery == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(b.ctx, defaultTimeout)
	instances, err := b.opts.Discovery.Services(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(b.ctx, defaultTimeout)
	watcher, err := b.opts.Discovery.Watch(ctx, cluster.Mesh.String())
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
	var skipped bool

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

		if skipped {
			b.updateInstances(instances)
		} else {
			skipped = true
		}
	}
}

// updateInstances 更新服务实例
func (b *Builder) updateInstances(instances []*registry.ServiceInstance) {
	for _, r := range b.resolvers {
		r.(iresolver.Builder).UpdateStates(instances)
	}
}

// Build 构建连接
func (b *Builder) Build(target string) (*grpc.ClientConn, error) {
	if b.err != nil {
		return nil, b.err
	}

	if c, ok := b.connections.Load(target); ok {
		return c.(*grpc.ClientConn), nil
	}

	c, err, _ := b.sfg.Do(target, func() (any, error) {
		cc, err := grpc.NewClient(target, b.dialOpts...)
		if err != nil {
			return nil, err
		}

		b.connections.Store(target, cc)

		return cc, nil
	})
	if err != nil {
		return nil, err
	}

	return c.(*grpc.ClientConn), nil
}
