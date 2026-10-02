package grpc

import (
	"github.com/dobyte/due/transport/grpc/v2/internal/client"
	"github.com/dobyte/due/transport/grpc/v2/internal/server"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/def"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc"
)

const (
	defaultServerAddr     = ":0"               // Default server address
	defaultClientDispatch = cluster.RoundRobin // Default client request dispatch (load balancing) strategy
)

const (
	defaultServerAddrKey       = "etc.transport.grpc.server.addr"
	defaultServerExposeKey     = "etc.transport.grpc.server.expose"
	defaultServerKeyFileKey    = "etc.transport.grpc.server.keyFile"
	defaultServerCertFileKey   = "etc.transport.grpc.server.certFile"
	defaultClientCAFileKey     = "etc.transport.grpc.client.caFile"
	defaultClientServerNameKey = "etc.transport.grpc.client.serverName"
	defaultClientDispatchKey   = "etc.transport.grpc.client.dispatch"
)

// Dispatch is the request dispatch strategy.
type Dispatch = def.Dispatch

const (
	Random             = def.Random             // Random
	RoundRobin         = def.RoundRobin         // Round-robin
	WeightedRoundRobin = def.WeightedRoundRobin // Weighted round-robin
	ConsistentHash     = def.ConsistentHash     // Consistent hash
)

type Option func(o *options)

type options struct {
	server server.Options
	client client.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = etc.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.server.Expose = etc.Get(defaultServerExposeKey).Bool()
	opts.server.KeyFile = etc.Get(defaultServerKeyFileKey).String()
	opts.server.CertFile = etc.Get(defaultServerCertFileKey).String()
	opts.client.CAFile = etc.Get(defaultClientCAFileKey).String()
	opts.client.ServerName = etc.Get(defaultClientServerNameKey).String()
	opts.client.Dispatch = Dispatch(etc.Get(defaultClientDispatchKey, defaultClientDispatch).String())

	return opts
}

// WithServerAddr sets the server listening address.
func WithServerAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}

// WithServerExpose sets whether to expose the internal communication address to the public network.
func WithServerExpose(expose bool) Option {
	return func(o *options) { o.server.Expose = expose }
}

// WithServerCredentials sets the server certificate and key.
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.server.CertFile, o.server.KeyFile = certFile, keyFile }
}

// WithServerOptions sets the server options.
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) { o.server.ServerOpts = opts }
}

// WithClientCredentials sets the client certificate and the server name to verify.
func WithClientCredentials(caFile string, serverName string) Option {
	return func(o *options) { o.client.CAFile, o.client.ServerName = caFile, serverName }
}

// WithClientDispatch sets the client request dispatch (load balancing) strategy.
func WithClientDispatch(dispatch Dispatch) Option {
	return func(o *options) { o.client.Dispatch = dispatch }
}

// WithClientDiscovery sets the client service discovery component.
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) { o.client.Discovery = discovery }
}

// WithClientDialOptions sets the client dial options.
func WithClientDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) { o.client.DialOpts = opts }
}
