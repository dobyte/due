package consul

import (
	"context"
	"github.com/dobyte/due/v2/config"
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
	defaultAddrKey                           = "config.registry.consul.addr"
	defaultHealthCheckKey                    = "config.registry.consul.healthCheck"
	defaultHealthCheckIntervalKey            = "config.registry.consul.healthCheckInterval"
	defaultHealthCheckTimeoutKey             = "config.registry.consul.healthCheckTimeout"
	defaultHeartbeatCheckKey                 = "config.registry.consul.heartbeatCheck"
	defaultHeartbeatCheckIntervalKey         = "config.registry.consul.heartbeatCheckInterval"
	defaultDeregisterCriticalServiceAfterKey = "config.registry.consul.deregisterCriticalServiceAfter"
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
		addr:                           config.Get(defaultAddrKey, defaultAddr).String(),
		enableHealthCheck:              config.Get(defaultHealthCheckKey, defaultHealthCheck).Bool(),
		healthCheckInterval:            config.Get(defaultHealthCheckIntervalKey, defaultHealthCheckInterval).Int(),
		healthCheckTimeout:             config.Get(defaultHealthCheckTimeoutKey, defaultHealthCheckTimeout).Int(),
		enableHeartbeatCheck:           config.Get(defaultHeartbeatCheckKey, defaultHeartbeatCheck).Bool(),
		heartbeatCheckInterval:         config.Get(defaultHeartbeatCheckIntervalKey, defaultHeartbeatCheckInterval).Int(),
		deregisterCriticalServiceAfter: config.Get(defaultDeregisterCriticalServiceAfterKey, defaultDeregisterCriticalServiceAfter).Int(),
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
