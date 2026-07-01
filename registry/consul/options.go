package consul

import (
	"context"
	"time"

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
	defaultTimeout                        = "3s"
	defaultRetryTimes                     = 3
	defaultRetryInterval                  = "10s"
)

const (
	defaultAddrKey                           = "etc.registry.consul.addr"
	defaultHealthCheckKey                    = "etc.registry.consul.healthCheck"
	defaultHealthCheckIntervalKey            = "etc.registry.consul.healthCheckInterval"
	defaultHealthCheckTimeoutKey             = "etc.registry.consul.healthCheckTimeout"
	defaultHeartbeatCheckKey                 = "etc.registry.consul.heartbeatCheck"
	defaultHeartbeatCheckIntervalKey         = "etc.registry.consul.heartbeatCheckInterval"
	defaultDeregisterCriticalServiceAfterKey = "etc.registry.consul.deregisterCriticalServiceAfter"
	defaultTimeoutKey                        = "etc.registry.consul.timeout"
	defaultRetryTimesKey                     = "etc.registry.consul.retryTimes"
	defaultRetryIntervalKey                  = "etc.registry.consul.retryInterval"
)

type Option func(o *options)

type options struct {
	addr string

	client *api.Client

	ctx context.Context

	enableHealthCheck bool

	healthCheckInterval int

	healthCheckTimeout int

	enableHeartbeatCheck bool

	heartbeatCheckInterval int

	deregisterCriticalServiceAfter int

	timeout time.Duration

	retryTimes int

	retryInterval time.Duration
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
		timeout:                        etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		retryTimes:                     etc.Get(defaultRetryTimesKey, defaultRetryTimes).Int(),
		retryInterval:                  etc.Get(defaultRetryIntervalKey, defaultRetryInterval).Duration(),
	}
}

func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

func WithClient(client *api.Client) Option {
	return func(o *options) { o.client = client }
}

func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

func WithEnableHealthCheck(enable bool) Option {
	return func(o *options) { o.enableHealthCheck = enable }
}

func WithHealthCheckInterval(interval int) Option {
	return func(o *options) { o.healthCheckInterval = interval }
}

func WithHealthCheckTimeout(timeout int) Option {
	return func(o *options) { o.healthCheckTimeout = timeout }
}

func WithEnableHeartbeatCheck(enable bool) Option {
	return func(o *options) { o.enableHeartbeatCheck = enable }
}

func WithHeartbeatCheckInterval(interval int) Option {
	return func(o *options) { o.heartbeatCheckInterval = interval }
}

func WithDeregisterCriticalServiceAfter(after int) Option {
	return func(o *options) { o.deregisterCriticalServiceAfter = after }
}

func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

func WithRetryTimes(retryTimes int) Option {
	return func(o *options) { o.retryTimes = retryTimes }
}

func WithRetryInterval(retryInterval time.Duration) Option {
	return func(o *options) { o.retryInterval = retryInterval }
}
