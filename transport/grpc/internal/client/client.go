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

type Client struct {
	conn grpc.ClientConnInterface
}

type Options struct {
	CertFile   string
	ServerName string
	Discovery  registry.Discovery
	ClientOpts []grpc.DialOption
}

func Dial(target string, opts *Options) (*grpc.ClientConn, error) {
	var err error
	var creds credentials.TransportCredentials

	if opts.CertFile != "" && opts.ServerName != "" {
		creds, err = credentials.NewClientTLSFromFile(opts.CertFile, opts.ServerName)
		if err != nil {
			return nil, err
		}
	} else {
		creds = insecure.NewCredentials()
	}

	resolvers := make([]resolver.Builder, 0, 2)
	resolvers = append(resolvers, direct.NewBuilder())
	if opts.Discovery != nil {
		resolvers = append(resolvers, discovery.NewBuilder(opts.Discovery))
	}

	clientOpts := make([]grpc.DialOption, 0, len(opts.ClientOpts)+2)
	clientOpts = append(clientOpts, grpc.WithTransportCredentials(creds))
	clientOpts = append(clientOpts, grpc.WithResolvers(resolvers...))

	return grpc.Dial(target, clientOpts...)
}
