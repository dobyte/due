package client

import (
	"github.com/dobyte/due/transport/grpc/v2/internal/resolver/direct"
	"github.com/dobyte/due/transport/grpc/v2/internal/resolver/discovery"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"sync"
)

type Builder struct {
	err         error
	opts        *Options
	dialOpts    []grpc.DialOption
	connections sync.Map
}

type Options struct {
	CertFile   string
	ServerName string
	Discovery  registry.Discovery
	DialOpts   []grpc.DialOption
}

func NewBuilder(opts *Options) *Builder {
	b := &Builder{opts: opts}

	var cred credentials.TransportCredentials
	if opts.CertFile != "" && opts.ServerName != "" {
		if cred, b.err = credentials.NewClientTLSFromFile(opts.CertFile, opts.ServerName); b.err != nil {
			return b
		}
	} else {
		cred = insecure.NewCredentials()
	}

	resolvers := make([]resolver.Builder, 0, 2)
	resolvers = append(resolvers, direct.NewBuilder(opts.Discovery))
	if opts.Discovery != nil {
		resolvers = append(resolvers, discovery.NewBuilder(opts.Discovery))
	}

	b.dialOpts = make([]grpc.DialOption, 0, len(opts.DialOpts)+2)
	b.dialOpts = append(b.dialOpts, opts.DialOpts...)
	b.dialOpts = append(b.dialOpts, grpc.WithTransportCredentials(cred))
	b.dialOpts = append(b.dialOpts, grpc.WithResolvers(resolvers...))

	return b
}

// Build 构建连接
func (b *Builder) Build(target string) (*grpc.ClientConn, error) {
	c, ok := b.connections.Load(target)
	if ok {
		return c.(*grpc.ClientConn), nil
	}

	cc, err := grpc.NewClient(target, b.dialOpts...)
	if err != nil {
		return nil, err
	}

	if c, ok = b.connections.LoadOrStore(target, cc); ok {
		_ = cc.Close()

		return c.(*grpc.ClientConn), nil
	} else {
		return cc, nil
	}
}
