package consul

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/hashicorp/consul/api"
)

const (
	defaultAddr                           = "127.0.0.1:8500"
	defaultTimeout                        = "3s"
	defaultRetryTimes                     = 3
	defaultHealthCheck                    = true
	defaultHealthCheckInterval            = 10
	defaultHealthCheckTimeout             = 5
	defaultHeartbeatCheck                 = true
	defaultHeartbeatCheckInterval         = 10
	defaultDeregisterCriticalServiceAfter = 30
)

const (
	defaultAddrKey                           = "etc.registry.consul.addr"
	defaultTimeoutKey                        = "etc.registry.consul.timeout"
	defaultRetryTimesKey                     = "etc.registry.consul.retryTimes"
	defaultHealthCheckKey                    = "etc.registry.consul.healthCheck"
	defaultHealthCheckIntervalKey            = "etc.registry.consul.healthCheckInterval"
	defaultHealthCheckTimeoutKey             = "etc.registry.consul.healthCheckTimeout"
	defaultHeartbeatCheckKey                 = "etc.registry.consul.heartbeatCheck"
	defaultHeartbeatCheckIntervalKey         = "etc.registry.consul.heartbeatCheckInterval"
	defaultDeregisterCriticalServiceAfterKey = "etc.registry.consul.deregisterCriticalServiceAfter"
)

type Option func(o *options)

type options struct {
	// Consul 地址
	// 默认值 127.0.0.1:8500
	addr string

	// Consul 客户端
	// 默认值 nil
	client *api.Client

	// 超时时间
	// 默认值 3s
	timeout time.Duration

	// 异常重试次数
	// 默认值 3
	retryTimes int

	// 是否开启健康检查
	// 默认值 true
	enableHealthCheck bool

	// 健康检查间隔
	// 默认值 10s
	healthCheckInterval int

	// 健康检查超时时间
	// 默认值 5s
	healthCheckTimeout int

	// 是否开启心跳检查
	// 默认值 true
	enableHeartbeatCheck bool

	// 心跳检查间隔
	// 默认值 10s
	heartbeatCheckInterval int

	// 注册服务后，等待多少秒后，自动注销服务
	// 默认值 30s
	deregisterCriticalServiceAfter int
}

func defaultOptions() *options {
	return &options{
		addr:                           etc.Get(defaultAddrKey, defaultAddr).String(),
		timeout:                        etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		retryTimes:                     etc.Get(defaultRetryTimesKey, defaultRetryTimes).Int(),
		enableHealthCheck:              etc.Get(defaultHealthCheckKey, defaultHealthCheck).Bool(),
		healthCheckInterval:            etc.Get(defaultHealthCheckIntervalKey, defaultHealthCheckInterval).Int(),
		healthCheckTimeout:             etc.Get(defaultHealthCheckTimeoutKey, defaultHealthCheckTimeout).Int(),
		enableHeartbeatCheck:           etc.Get(defaultHeartbeatCheckKey, defaultHeartbeatCheck).Bool(),
		heartbeatCheckInterval:         etc.Get(defaultHeartbeatCheckIntervalKey, defaultHeartbeatCheckInterval).Int(),
		deregisterCriticalServiceAfter: etc.Get(defaultDeregisterCriticalServiceAfterKey, defaultDeregisterCriticalServiceAfter).Int(),
	}
}

// WithAddr 设置 Consul 地址
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithClient 设置 Consul 客户端
func WithClient(client *api.Client) Option {
	return func(o *options) { o.client = client }
}

// WithTimeout 设置客户端连接超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithRetryTimes 设置异常重试次数
func WithRetryTimes(retryTimes int) Option {
	return func(o *options) { o.retryTimes = retryTimes }
}

// WithEnableHealthCheck 设置是否开启健康检查
func WithEnableHealthCheck(enable bool) Option {
	return func(o *options) { o.enableHealthCheck = enable }
}

// WithHealthCheckInterval 设置健康检查间隔
func WithHealthCheckInterval(interval int) Option {
	return func(o *options) { o.healthCheckInterval = interval }
}

// WithHealthCheckTimeout 设置健康检查超时时间
func WithHealthCheckTimeout(timeout int) Option {
	return func(o *options) { o.healthCheckTimeout = timeout }
}

// WithEnableHeartbeatCheck 设置是否开启心跳检查
func WithEnableHeartbeatCheck(enable bool) Option {
	return func(o *options) { o.enableHeartbeatCheck = enable }
}

// WithHeartbeatCheckInterval 设置心跳检查间隔
func WithHeartbeatCheckInterval(interval int) Option {
	return func(o *options) { o.heartbeatCheckInterval = interval }
}

// WithDeregisterCriticalServiceAfter 设置注册服务后，等待多少秒后，自动注销服务
func WithDeregisterCriticalServiceAfter(after int) Option {
	return func(o *options) { o.deregisterCriticalServiceAfter = after }
}
