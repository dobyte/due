package tcp

import (
	"github.com/dobyte/due/v2/etc"
	"time"
)

const (
	defaultServerAddr                    = ":3553"
	defaultServerMaxConnNum              = 5000
	defaultServerHeartbeatInterval       = "10s"
	defaultServerHeartbeatMechanism      = RespHeartbeat
	defaultServerHeartbeatWithServerTime = true
)

const (
	defaultServerAddrKey                    = "etc.network.tcp.server.addr"
	defaultServerMaxConnNumKey              = "etc.network.tcp.server.maxConnNum"
	defaultServerHeartbeatIntervalKey       = "etc.network.tcp.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey      = "etc.network.tcp.server.heartbeatMechanism"
	defaultServerHeartbeatWithServerTimeKey = "etc.network.tcp.server.heartbeatWithServerTime"
)

const (
	RespHeartbeat HeartbeatMechanism = "resp" // 响应式心跳
	TickHeartbeat HeartbeatMechanism = "tick" // 主动定时心跳
)

type HeartbeatMechanism string

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr                    string             // 监听地址，默认0.0.0.0:3553
	maxConnNum              int                // 最大连接数，默认5000
	heartbeatInterval       time.Duration      // 心跳检测间隔时间，默认10s
	heartbeatMechanism      HeartbeatMechanism // 心跳机制，默认resp
	heartbeatWithServerTime bool               // 下行心跳是否携带服务器时间，默认为true
}

func defaultServerOptions() *serverOptions {
	return &serverOptions{
		addr:                    etc.Get(defaultServerAddrKey, defaultServerAddr).String(),
		maxConnNum:              etc.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(),
		heartbeatInterval:       etc.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration(),
		heartbeatMechanism:      HeartbeatMechanism(etc.Get(defaultServerHeartbeatMechanismKey, defaultServerHeartbeatMechanism).String()),
		heartbeatWithServerTime: etc.Get(defaultServerHeartbeatWithServerTimeKey, defaultServerHeartbeatWithServerTime).Bool(),
	}
}

// WithServerListenAddr 设置监听地址
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) { o.addr = addr }
}

// WithServerMaxConnNum 设置连接的最大连接数
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) { o.maxConnNum = maxConnNum }
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatInterval = heartbeatInterval }
}

// WithServerHeartbeatMechanism 设置心跳机制
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) { o.heartbeatMechanism = heartbeatMechanism }
}

// WithServerHeartbeatWithServerTime 设置下行心跳是否携带服务器时间
func WithServerHeartbeatWithServerTime(heartbeatWithServerTime bool) ServerOption {
	return func(o *serverOptions) { o.heartbeatWithServerTime = heartbeatWithServerTime }
}
