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

// Option configures the service registry and discovery component.
type Option func(o *options)

type options struct {
	// Consul address.
	// Defaults to 127.0.0.1:8500.
	addr string

	// Consul client.
	// Defaults to nil.
	client *api.Client

	// Timeout.
	// Defaults to 3s.
	timeout time.Duration

	// Number of retries on errors.
	// Defaults to 3.
	retryTimes int

	// Whether to enable health checks.
	// Defaults to true.
	enableHealthCheck bool

	// Health check interval.
	// Defaults to 10s.
	healthCheckInterval int

	// Health check timeout.
	// Defaults to 5s.
	healthCheckTimeout int

	// Whether to enable heartbeat checks.
	// Defaults to true.
	enableHeartbeatCheck bool

	// Heartbeat check interval.
	// Defaults to 10s.
	heartbeatCheckInterval int

	// Number of seconds to wait after registering a service before automatically deregistering it.
	// Defaults to 30s.
	deregisterCriticalServiceAfter int
}

// defaultOptions returns the options with their default values.
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

// WithAddr sets the Consul address.
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithClient sets the Consul client.
func WithClient(client *api.Client) Option {
	return func(o *options) { o.client = client }
}

// WithTimeout sets the client connection timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithRetryTimes sets the number of retries on errors.
func WithRetryTimes(retryTimes int) Option {
	return func(o *options) { o.retryTimes = retryTimes }
}

// WithEnableHealthCheck sets whether to enable health checks.
func WithEnableHealthCheck(enable bool) Option {
	return func(o *options) { o.enableHealthCheck = enable }
}

// WithHealthCheckInterval sets the health check interval.
func WithHealthCheckInterval(interval int) Option {
	return func(o *options) { o.healthCheckInterval = interval }
}

// WithHealthCheckTimeout sets the health check timeout.
func WithHealthCheckTimeout(timeout int) Option {
	return func(o *options) { o.healthCheckTimeout = timeout }
}

// WithEnableHeartbeatCheck sets whether to enable heartbeat checks.
func WithEnableHeartbeatCheck(enable bool) Option {
	return func(o *options) { o.enableHeartbeatCheck = enable }
}

// WithHeartbeatCheckInterval sets the heartbeat check interval.
func WithHeartbeatCheckInterval(interval int) Option {
	return func(o *options) { o.heartbeatCheckInterval = interval }
}

// WithDeregisterCriticalServiceAfter sets the number of seconds to wait after registering a service
// before automatically deregistering it.
func WithDeregisterCriticalServiceAfter(after int) Option {
	return func(o *options) { o.deregisterCriticalServiceAfter = after }
}
