package tcp

import (
	"github.com/dobyte/due/v2/etc"
	"time"
)

const (
	defaultClientDialAddr          = "127.0.0.1:3553"
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientDialAddrKey          = "etc.network.tcp.client.addr"
	defaultClientHeartbeatIntervalKey = "etc.network.tcp.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		addr:              etc.Get(defaultClientDialAddrKey, defaultClientDialAddr).String(),
		heartbeatInterval: etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(),
	}
}

// WithClientDialAddr 设置拨号地址
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
