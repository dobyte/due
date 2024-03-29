package netpoll

import (
	"github.com/symsimmy/due/config"
	"time"
)

const (
	defaultServerAddr              = ":3553"
	defaultServerMaxConnNum        = 500000
	defaultServerHeartbeatInterval = 10
)

const (
	defaultServerAddrKey              = "etc.network.tcp.server.addr"
	defaultServerMaxConnNumKey        = "etc.network.tcp.server.maxConnNum"
	defaultServerHeartbeatIntervalKey = "etc.network.tcp.server.heartbeatInterval"
)

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr              string        // 监听地址，默认0.0.0.0:3553
	maxConnNum        int           // 最大连接数，默认5000
	heartbeatInterval time.Duration // 心跳检测间隔时间，默认10s
}

func defaultServerOptions() *serverOptions {
	return &serverOptions{
		addr:              config.Get(defaultServerAddrKey, defaultServerAddr).String(),
		maxConnNum:        config.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(),
		heartbeatInterval: config.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration() * time.Second,
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
