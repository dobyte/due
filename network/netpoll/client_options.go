package netpoll

import (
	"github.com/dobyte/due/v2/etc"
	"time"
)

const (
	defaultClientDialAddr          = "127.0.0.1:3553"
	defaultClientDialTimeout       = "5s"
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientDialAddrKey          = "etc.network.tcp.client.addr"
	defaultClientDialTimeoutKey       = "etc.network.tcp.client.timeout"
	defaultClientHeartbeatIntervalKey = "etc.network.tcp.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	timeout           time.Duration // 拨号超时时间，默认5s
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		addr:              etc.Get(defaultClientDialAddrKey, defaultClientDialAddr).String(),
		timeout:           etc.Get(defaultClientDialTimeoutKey, defaultClientDialTimeout).Duration(),
		heartbeatInterval: etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(),
	}
}

// WithClientDialAddr 设置拨号地址
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientDialTimeout 设置拨号超时时间
func WithClientDialTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.timeout = timeout }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
