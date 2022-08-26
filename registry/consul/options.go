package consul

import "context"

type Option func(o *options)

type options struct {
	ctx                            context.Context // context
	address                        string          // consul地址，默认127.0.0.1:8500
	enableHealthCheck              bool            // 是否启用健康检查
	healthCheckInterval            int             // 健康检查时间间隔，默认10秒
	healthCheckTimeout             int             // 健康检查超时时间，默认5秒
	enableHeartbeatCheck           bool            // 是否启用心跳检查
	heartbeatCheckInterval         int             // 心跳检查时间间隔，默认10秒
	deregisterCriticalServiceAfter int             // 健康检测失败后自动注销服务时间，默认30秒
}

// WithContext 设置context
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithAddress 设置consul地址
func WithAddress(address string) Option {
	return func(o *options) { o.address = address }
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
