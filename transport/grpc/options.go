package grpc

import (
	"github.com/symsimmy/due/config"
	"github.com/symsimmy/due/registry"
	"github.com/symsimmy/due/transport/grpc/internal/client"
	"github.com/symsimmy/due/transport/grpc/internal/server"
	"google.golang.org/grpc"
)

const (
	defaultServerAddr                                          = ":0" // 默认服务器地址
	defaultClientPoolSize                                      = 10   // 默认客户端连接池大小
	defaultServerKeepAliveEnforcementPolicyMinTime             = 5    // If a client pings more than once every 5 seconds, terminate the connection
	defaultServerKeepAliveEnforcementPolicyPermitWithoutStream = true // Allow pings even when there are no active streams
	defaultServerKeepAliveMaxConnectionIdle                    = -1   // If a client is idle for x seconds, send a GOAWAY
	defaultServerKeepAliveMaxConnectionAge                     = -1   // If any connection is alive for more than x seconds, send a GOAWAY
	defaultServerKeepAliveMaxConnectionAgeGrace                = 5    // Allow 5 seconds for pending RPCs to complete before forcibly closing connections
	defaultServerKeepAliveTime                                 = 5    // Ping the client if it is idle for 5 seconds to ensure the connection is still active
	defaultServerKeepAliveTimeout                              = 1    // Wait 1 second for the ping ack before assuming the connection is dead
	defaultClientKeepAliveTime                                 = 10   // send pings every 10 seconds if there is no activity
	defaultClientKeepAliveTimeout                              = 1    // wait 1 second for ping ack before considering the connection dead
	defaultClientKeepAlivePermitWithoutStream                  = true // send pings even without active streams
)

const (
	defaultServerAddrKey                                          = "config.transport.grpc.server.addr"
	defaultServerHostAddrKey                                      = "config.transport.grpc.server.hostAddr"
	defaultServerKeyFileKey                                       = "config.transport.grpc.server.keyFile"
	defaultServerCertFileKey                                      = "config.transport.grpc.server.certFile"
	defaultServerKeepAliveEnforcementPolicyMinTimeKey             = "config.transport.grpc.server.keepAlive.minTime"
	defaultServerKeepAliveEnforcementPolicyPermitWithoutStreamKey = "config.transport.grpc.server.keepAlive.permitWithoutStream"
	defaultServerKeepAliveMaxConnectionIdleKey                    = "config.transport.grpc.server.keepAlive.MaxConnectionIdle"
	defaultServerKeepAliveMaxConnectionAgeKey                     = "config.transport.grpc.server.keepAlive.MaxConnectionAge"
	defaultServerKeepAliveMaxConnectionAgeGraceKey                = "config.transport.grpc.server.keepAlive.MaxConnectionAgeGrace"
	defaultServerKeepAliveTimeKey                                 = "config.transport.grpc.server.keepAlive.Time"
	defaultServerKeepAliveTimeoutKey                              = "config.transport.grpc.server.keepAlive.Timeout"

	defaultClientPoolSizeKey                     = "config.transport.grpc.client.poolSize"
	defaultClientCertFileKey                     = "config.transport.grpc.client.certFile"
	defaultClientServerNameKey                   = "config.transport.grpc.client.serverName"
	defaultClientKeepAliveTimeKey                = "config.transport.grpc.client.keepAlive.time"
	defaultClientKeepAliveTimeoutKey             = "config.transport.grpc.client.keepAlive.timeout"
	defaultClientKeepAlivePermitWithoutStreamKey = "config.transport.grpc.client.keepAlive.permitWithoutStream"
)

type Option func(o *options)

type options struct {
	server server.Options
	client client.Options
}

func defaultOptions() *options {
	opts := &options{}
	opts.server.Addr = config.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.server.HostAddr = config.Get(defaultServerHostAddrKey).String()
	opts.server.KeyFile = config.Get(defaultServerKeyFileKey).String()
	opts.server.CertFile = config.Get(defaultServerCertFileKey).String()
	opts.server.KeepAliveEnforcementPolicyMinTime = config.Get(defaultServerKeepAliveEnforcementPolicyMinTimeKey, defaultServerKeepAliveEnforcementPolicyMinTime).Int()
	opts.server.KeepAliveEnforcementPolicyPermitWithoutStream = config.Get(defaultServerKeepAliveEnforcementPolicyPermitWithoutStreamKey, defaultServerKeepAliveEnforcementPolicyPermitWithoutStream).Bool()
	opts.server.KeepAliveMaxConnectionIdle = config.Get(defaultServerKeepAliveMaxConnectionIdleKey, defaultServerKeepAliveMaxConnectionIdle).Int()
	opts.server.KeepAliveMaxConnectionAge = config.Get(defaultServerKeepAliveMaxConnectionAgeKey, defaultServerKeepAliveMaxConnectionAge).Int()
	opts.server.KeepAliveMaxConnectionAgeGrace = config.Get(defaultServerKeepAliveMaxConnectionAgeGraceKey, defaultServerKeepAliveMaxConnectionAgeGrace).Int()
	opts.server.KeepAliveTime = config.Get(defaultServerKeepAliveTimeKey, defaultServerKeepAliveTime).Int()
	opts.server.KeepAliveTimeout = config.Get(defaultServerKeepAliveTimeoutKey, defaultServerKeepAliveTimeout).Int()

	opts.client.PoolSize = config.Get(defaultClientPoolSizeKey, defaultClientPoolSize).Int()
	opts.client.CertFile = config.Get(defaultClientCertFileKey).String()
	opts.client.ServerName = config.Get(defaultClientServerNameKey).String()
	opts.client.KeepAliveTime = config.Get(defaultClientKeepAliveTimeKey, defaultClientKeepAliveTime).Int()
	opts.client.KeepAliveTimeout = config.Get(defaultClientKeepAliveTimeoutKey, defaultClientKeepAliveTimeout).Int()
	opts.client.KeepAlivePermitWithoutStream = config.Get(defaultClientKeepAlivePermitWithoutStreamKey, defaultClientKeepAlivePermitWithoutStream).Bool()

	return opts
}

// WithServerListenAddr 设置服务器监听地址
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}

// WithServerCredentials 设置服务器证书和秘钥
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.server.KeyFile, o.server.CertFile = keyFile, certFile }
}

// WithServerOptions 设置服务器选项
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) { o.server.ServerOpts = opts }
}

// WithClientPoolSize 设置客户端连接池大小
func WithClientPoolSize(size int) Option {
	return func(o *options) { o.client.PoolSize = size }
}

// WithClientCredentials 设置客户端证书和校验域名
func WithClientCredentials(certFile string, serverName string) Option {
	return func(o *options) { o.client.CertFile, o.client.ServerName = certFile, serverName }
}

// WithClientDiscovery 设置客户端服务发现组件
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) { o.client.Discovery = discovery }
}

// WithClientKeepAliveParams 设置客户端keep alive参数
func WithClientKeepAliveParams(time int, timeout int, permitWithoutStream bool) Option {
	return func(o *options) {
		o.client.KeepAliveTime, o.client.KeepAliveTimeout, o.client.KeepAlivePermitWithoutStream = time, timeout, permitWithoutStream
	}
}

// WithClientDialOptions 设置客户端拨号选项
func WithClientDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) { o.client.DialOpts = opts }
}
