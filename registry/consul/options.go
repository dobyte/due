package consul

import (
	"context"
	"github.com/dobyte/due/v2/etc"
	"github.com/hashicorp/consul/api"
)

const (
	defaultAddr                           = "127.0.0.1:8500"
	defaultHealthCheck                    = true
	defaultHealthCheckInterval            = 10
	defaultHealthCheckTimeout             = 5
	defaultHeartbeatCheck                 = true
	defaultHeartbeatCheckInterval         = 10
	defaultDeregisterCriticalServiceAfter = 30
)

const (
	defaultAddrKey                           = "etc.registry.consul.addr"
	defaultHealthCheckKey                    = "etc.registry.consul.healthCheck"
	defaultHealthCheckIntervalKey            = "etc.registry.consul.healthCheckInterval"
	defaultHealthCheckTimeoutKey             = "etc.registry.consul.healthCheckTimeout"
	defaultHeartbeatCheckKey                 = "etc.registry.consul.heartbeatCheck"
	defaultHeartbeatCheckIntervalKey         = "etc.registry.consul.heartbeatCheckInterval"
	defaultDeregisterCriticalServiceAfterKey = "etc.registry.consul.deregisterCriticalServiceAfter"
)

type Option func(o *options)

type options struct {
	// 客户端连接地址
	// 内建客户端配置，默认为127.0.0.1:8500
	addr string

	// 外部客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client *api.Client

	// 上下文
	// 默认为context.Background
	ctx context.Context

	// 是否启用健康检查
	// 默认为true
	enableHealthCheck bool

	// 健康检查时间间隔（秒），仅在启用健康检查后生效
	// 默认10秒
	healthCheckInterval int

	// 健康检查超时时间（秒），仅在启用健康检查后生效
	// 默认5秒
	healthCheckTimeout int

	// 是否启用心跳检查
	// 默认为true
	enableHeartbeatCheck bool

	// 心跳检查时间间隔（秒），仅在启用心跳检查后生效
	// 默认10秒
	heartbeatCheckInterval int

	// 健康检测失败后自动注销服务时间（秒）
	// 默认30秒
	deregisterCriticalServiceAfter int
}

func defaultOptions() *options {
	return &options{
		ctx:                            context.Background(),
		addr:                           etc.Get(defaultAddrKey, defaultAddr).String(),
		enableHealthCheck:              etc.Get(defaultHealthCheckKey, defaultHealthCheck).Bool(),
		healthCheckInterval:            etc.Get(defaultHealthCheckIntervalKey, defaultHealthCheckInterval).Int(),
		healthCheckTimeout:             etc.Get(defaultHealthCheckTimeoutKey, defaultHealthCheckTimeout).Int(),
		enableHeartbeatCheck:           etc.Get(defaultHeartbeatCheckKey, defaultHeartbeatCheck).Bool(),
		heartbeatCheckInterval:         etc.Get(defaultHeartbeatCheckIntervalKey, defaultHeartbeatCheckInterval).Int(),
		deregisterCriticalServiceAfter: etc.Get(defaultDeregisterCriticalServiceAfterKey, defaultDeregisterCriticalServiceAfter).Int(),
	}
}

// WithAddr 设置客户端连接地址
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithClient 设置外部客户端
func WithClient(client *api.Client) Option {
	return func(o *options) { o.client = client }
}

// WithContext 设置context
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithEnableHealthCheck 设置是否启用健康检查
func WithEnableHealthCheck(enable bool) Option {
	return func(o *options) { o.enableHealthCheck = enable }
}

// WithHealthCheckInterval 设置健康检查时间间隔
func WithHealthCheckInterval(interval int) Option {
	return func(o *options) { o.healthCheckInterval = interval }
}

// WithHealthCheckTimeout 设置健康检查超时时间
func WithHealthCheckTimeout(timeout int) Option {
	return func(o *options) { o.healthCheckTimeout = timeout }
}

// WithEnableHeartbeatCheck 设置是否启用心跳检查
func WithEnableHeartbeatCheck(enable bool) Option {
	return func(o *options) { o.enableHeartbeatCheck = enable }
}

// WithHeartbeatCheckInterval 设置心跳检查时间间隔
func WithHeartbeatCheckInterval(interval int) Option {
	return func(o *options) { o.heartbeatCheckInterval = interval }
}

// WithDeregisterCriticalServiceAfter 设置健康检测失败后自动注销服务时间
func WithDeregisterCriticalServiceAfter(after int) Option {
	return func(o *options) { o.deregisterCriticalServiceAfter = after }
}
