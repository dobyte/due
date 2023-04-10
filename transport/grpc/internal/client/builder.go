package client

import (
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/transport/grpc/internal/resolver/direct"
	"github.com/dobyte/due/transport/grpc/internal/resolver/discovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

type Builder struct {
	err      error
	dialOpts []grpc.DialOption
}

type Options struct {
	CertFile   string
	ServerName string
	Discovery  registry.Discovery
	DialOpts   []grpc.DialOption
}

func NewBuilder(opts *Options) *Builder {
	b := &Builder{}

	var creds credentials.TransportCredentials
	if opts.CertFile != "" && opts.ServerName != "" {
		creds, b.err = credentials.NewClientTLSFromFile(opts.CertFile, opts.ServerName)
		if b.err != nil {
			return b
		}
	} else {
		creds = insecure.NewCredentials()
	}

	resolvers := make([]resolver.Builder, 0, 2)
	resolvers = append(resolvers, direct.NewBuilder())
	if opts.Discovery != nil {
		resolvers = append(resolvers, discovery.NewBuilder(opts.Discovery))
	}

	b.dialOpts = make([]grpc.DialOption, 0, len(opts.DialOpts)+2)
	b.dialOpts = append(b.dialOpts, grpc.WithTransportCredentials(creds))
	b.dialOpts = append(b.dialOpts, grpc.WithResolvers(resolvers...))

	return b
}

// Build 构建连接
func (b *Builder) Build(target string) (*grpc.ClientConn, error) {
	if b.err != nil {
		return nil, b.err
	}

	return grpc.Dial(target, b.dialOpts...)
}
