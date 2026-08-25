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
	defaultServerAddr     = ":0"               // 默认服务器地址
	defaultClientPoolSize = 10                 // 默认客户端连接池大小
	defaultClientDispatch = cluster.RoundRobin // 默认客户端请求分发策略
	defaultClientFailMode = cli.Failtry        // 默认客户端故障模式
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

// Dispatch 分发策略
type Dispatch = def.Dispatch

const (
	Random             = def.Random             // 随机
	RoundRobin         = def.RoundRobin         // 轮询
	WeightedRoundRobin = def.WeightedRoundRobin // 加权轮询
	ConsistentHash     = def.ConsistentHash     // 一致性哈希分发
)

// Option 传输器配置项
type Option func(o *options)

// options 传输器配置
type options struct {
	server server.Options
	client client.Options
}

// defaultOptions 获取默认配置项，配置值优先从配置中心读取
// @return @1 *options 默认配置项
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

// WithServerAddr 设置服务器监听地址
// @param addr string 服务器监听地址
// @return @1 Option 配置项
func WithServerAddr(addr string) Option {
	return func(o *options) { o.server.Addr = addr }
}

// WithServerExpose 设置是否将内部通信地址暴露到公网
// @param expose bool 是否暴露到公网
// @return @1 Option 配置项
func WithServerExpose(expose bool) Option {
	return func(o *options) { o.server.Expose = expose }
}

// WithServerCredentials 设置服务器证书和秘钥
// @param certFile string 证书文件路径
// @param keyFile string 秘钥文件路径
// @return @1 Option 配置项
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.server.CertFile, o.server.KeyFile = certFile, keyFile }
}

// WithClientPoolSize 设置客户端连接池大小
// @param size int 连接池大小
// @return @1 Option 配置项
func WithClientPoolSize(size int) Option {
	return func(o *options) { o.client.PoolSize = size }
}

// WithClientCredentials 设置客户端证书和校验域名
// @param caFile string 证书文件路径
// @param serverName string 服务器校验域名
// @return @1 Option 配置项
func WithClientCredentials(caFile string, serverName string) Option {
	return func(o *options) { o.client.CAFile, o.client.ServerName = caFile, serverName }
}

// WithClientDiscovery 设置客户端服务发现组件
// @param discovery registry.Discovery 服务发现组件
// @return @1 Option 配置项
func WithClientDiscovery(discovery registry.Discovery) Option {
	return func(o *options) { o.client.Discovery = discovery }
}

// WithClientDispatch 设置客户端请求分发策略（负载均衡策略）
func WithClientDispatch(dispatch Dispatch) Option {
	return func(o *options) { o.client.Dispatch = dispatch }
}

// WithClientFailMode 设置客户端故障模式
func WithClientFailMode(failMode cli.FailMode) Option {
	return func(o *options) { o.client.FailMode = failMode }
}
