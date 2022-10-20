package tcp

import (
	"github.com/dobyte/due/config"
	"time"
)

const (
	defaultClientDialAddr          = "127.0.0.1:3553"
	defaultClientMaxMsgLen         = 1024 * 1024
	defaultClientHeartbeat         = false
	defaultClientHeartbeatInterval = 10
)

const (
	defaultClientDialAddrKey          = "config.network.tcp.client.addr"
	defaultClientMaxMsgLenKey         = "config.network.tcp.client.maxMsgLen"
	defaultClientHeartbeatKey         = "config.network.tcp.client.heartbeat"
	defaultClientHeartbeatIntervalKey = "config.network.tcp.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	maxMsgLen         int           // 最大消息长度
	enableHeartbeat   bool          // 是否启用心跳，默认不启用
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		addr:              config.Get(defaultClientDialAddrKey, defaultClientDialAddr).String(),
		maxMsgLen:         config.Get(defaultClientMaxMsgLenKey, defaultClientMaxMsgLen).Int(),
		enableHeartbeat:   config.Get(defaultClientHeartbeatKey, defaultClientHeartbeat).Bool(),
		heartbeatInterval: config.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration() * time.Second,
	}
}

// WithClientDialAddr 设置拨号地址
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientMaxMsgLen 设置消息最大长度
func WithClientMaxMsgLen(maxMsgLen int) ClientOption {
	return func(o *clientOptions) { o.maxMsgLen = maxMsgLen }
}

// WithClientEnableHeartbeat 设置是否启用心跳间隔时间
func WithClientEnableHeartbeat(enable bool) ClientOption {
	return func(o *clientOptions) { o.enableHeartbeat = enable }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
