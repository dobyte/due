package client

import (
	"github.com/symsimmy/due/registry"
	"github.com/symsimmy/due/transport/grpc/internal/resolver/direct"
	"github.com/symsimmy/due/transport/grpc/internal/resolver/discovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
	"sync"
	"time"
)

type Builder struct {
	err      error
	opts     *Options
	dialOpts []grpc.DialOption
	pools    sync.Map
}

type Options struct {
	PoolSize                     int
	CertFile                     string
	ServerName                   string
	Discovery                    registry.Discovery
	DialOpts                     []grpc.DialOption
	KeepAliveTime                int
	KeepAliveTimeout             int
	KeepAlivePermitWithoutStream bool
}

func NewBuilder(opts *Options) *Builder {
	b := &Builder{opts: opts}

	var creds credentials.TransportCredentials
	if opts.CertFile != "" && opts.ServerName != "" {
		creds, b.err = credentials.NewClientTLSFromFile(opts.CertFile, opts.ServerName)
		if b.err != nil {
			return b
		}
	} else {
		creds = insecure.NewCredentials()
	}

	var kacp keepalive.ClientParameters
	if opts.KeepAliveTime > 0 && opts.KeepAliveTimeout > 0 {
		kacp = keepalive.ClientParameters{
			Time:                time.Duration(opts.KeepAliveTime) * time.Second,    // send pings every opts.KeepAliveTime seconds if there is no activity
			Timeout:             time.Duration(opts.KeepAliveTimeout) * time.Second, // wait opts.KeepAliveTimeout second for ping ack before considering the connection dead
			PermitWithoutStream: opts.KeepAlivePermitWithoutStream,                  // if true, send pings even without active streams
		}
	}

	resolvers := make([]resolver.Builder, 0, 2)
	resolvers = append(resolvers, direct.NewBuilder())
	if opts.Discovery != nil {
		resolvers = append(resolvers, discovery.NewBuilder(opts.Discovery))
	}

	b.dialOpts = make([]grpc.DialOption, 0, len(opts.DialOpts)+3)
	b.dialOpts = append(b.dialOpts, grpc.WithTransportCredentials(creds))
	b.dialOpts = append(b.dialOpts, grpc.WithResolvers(resolvers...))
	if opts.KeepAliveTime > 0 && opts.KeepAliveTimeout > 0 {
		b.dialOpts = append(b.dialOpts, grpc.WithKeepaliveParams(kacp))
	}

	return b
}

// Build 构建连接
func (b *Builder) Build(target string) (*grpc.ClientConn, error) {
	if b.err != nil {
		return nil, b.err
	}

	val, ok := b.pools.Load(target)
	if ok {
		return val.(*Pool).Get(), nil
	}

	size := b.opts.PoolSize
	if size <= 0 {
		size = 10
	}

	pool, err := newPool(size, target, b.dialOpts...)
	if err != nil {
		return nil, err
	}

	b.pools.Store(target, pool)

	return pool.Get(), nil
}
