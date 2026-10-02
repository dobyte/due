package rpcx

import (
	"github.com/dobyte/due/transport/rpcx/v2/internal/client"
	"github.com/dobyte/due/transport/rpcx/v2/internal/server"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/def"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
)

const (
	defaultServerAddr     = ":0"               // Default server address
	defaultClientPoolSize = 10                 // Default client connection pool size
	defaultClientDispatch = cluster.RoundRobin // Default client request dispatch strategy
	defaultClientFailMode = cli.Failtry        // Default client fail mode
)

const (
	defaultServerAddrKey       = "etc.transport.rpcx.server.addr"
	defaultServerExposeKey     = "etc.transport.rpcx.server.expose"
	defaultServerKeyFileKey    = "etc.transport.rpcx.server.keyFile"
	defaultServerCertFileKey   = "etc.transport.rpcx.server.certFile"
	defaultClientPoolSizeKey   = "etc.transport.rpcx.client.poolSize"
	defaultClientCAFileKey     = "etc.transport.rpcx.client.caFile"
	defaultClientServerNameKey = "etc.transport.rpcx.client.serverName"
	defaultClientDispatchKey   = "etc.transport.rpcx.client.dispatch"
	defaultClientFailModeKey   = "etc.transport.rpcx.client.failMode"
)

// Dispatch is the request dispatch strategy.
type Dispatch = def.Dispatch

const (
	Random             = def.Random             // Random
	RoundRobin         = def.RoundRobin         // Round-robin
	WeightedRoundRobin = def.WeightedRoundRobin // Weighted round-robin
	ConsistentHash     = def.ConsistentHash     // Consistent hash
)

// Option is a transporter option.
type Option func(o *options)

// options holds the transporter options.
type options struct {
	server server.Options
	client client.Options
}

// defaultOptions returns the default options, reading values from the config center first.
func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = etc.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.server.Expose = etc.Get(defaultServerExposeKey).Bool()
	opts.server.KeyFile = etc.Get(defaultServerKeyFileKey).String()
	opts.server.CertFile = etc.Get(defaultServerCertFileKey).String()
	opts.client.PoolSize = etc.Get(defaultClientPoolSizeKey, defaultClientPoolSize).Int()
	opts.client.CAFile = etc.Get(defaultClientCAFileKey).String()
	opts.client.ServerName = etc.Get(defaultClientServerNameKey).String()
	opts.client.Dispatch = Dispatch(etc.Get(defaultClientDispatchKey, defaultClientDispatch).String())
	opts.client.FailMode = cli.FailMode(etc.Get(defaultClientFailModeKey, int(defaultClientFailMode)).Int())

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

// WithClientPoolSize sets the client connection pool size.
func WithClientPoolSize(size int) Option {
	return func(o *options) { o.client.PoolSize = size }
}

// WithClientCredentials sets the client certificate and the server name to verify.
func WithClientCredentials(caFile string, serverName string) Option {
	return func(o *options) { o.client.CAFile, o.client.ServerName = caFile, serverName }
}

// WithClientDiscovery sets the client service discovery component.
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) { o.client.Discovery = discovery }
}

// WithClientDispatch sets the client request dispatch (load balancing) strategy.
func WithClientDispatch(dispatch Dispatch) Option {
	return func(o *options) { o.client.Dispatch = dispatch }
}

// WithClientFailMode sets the client fail mode.
func WithClientFailMode(failMode cli.FailMode) Option {
	return func(o *options) { o.client.FailMode = failMode }
}
