package ws

import (
	"github.com/dobyte/due/config"
	"time"
)

const (
	defaultClientDialUrl           = "ws://127.0.0.1:3553"
	defaultClientMaxMsgLen         = 1024
	defaultClientHandshakeTimeout  = 10
	defaultClientHeartbeat         = false
	defaultClientHeartbeatInterval = 10
)

const (
	defaultClientDialUrlKey           = "config.network.ws.client.url"
	defaultClientMaxMsgLenKey         = "config.network.ws.client.maxMsgLen"
	defaultClientHandshakeTimeoutKey  = "config.network.ws.client.handshakeTimeout"
	defaultClientHeartbeatKey         = "config.network.ws.client.heartbeat"
	defaultClientHeartbeatIntervalKey = "config.network.ws.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	url               string        // 拨号地址
	maxMsgLen         int           // 最大消息长度（字节），默认1kb
	handshakeTimeout  time.Duration // 握手超时时间
	enableHeartbeat   bool          // 是否启用心跳，默认不启用
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		url:               config.Get(defaultClientDialUrlKey, defaultClientDialUrl).String(),
		maxMsgLen:         config.Get(defaultClientMaxMsgLenKey, defaultClientMaxMsgLen).Int(),
		handshakeTimeout:  config.Get(defaultClientHandshakeTimeoutKey, defaultClientHandshakeTimeout).Duration() * time.Second,
		enableHeartbeat:   config.Get(defaultClientHeartbeatKey, defaultClientHeartbeat).Bool(),
		heartbeatInterval: config.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration() * time.Second,
	}
}

// WithClientDialUrl 设置拨号链接
func WithClientDialUrl(url string) ClientOption {
	return func(o *clientOptions) { o.url = url }
}

// WithClientMaxMsgLen 设置消息最大长度
func WithClientMaxMsgLen(maxMsgLen int) ClientOption {
	return func(o *clientOptions) { o.maxMsgLen = maxMsgLen }
}

// WithClientHandshakeTimeout 设置握手超时时间
func WithClientHandshakeTimeout(handshakeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.handshakeTimeout = handshakeTimeout }
}

// WithClientEnableHeartbeat 设置是否启用心跳间隔时间
func WithClientEnableHeartbeat(enable bool) ClientOption {
	return func(o *clientOptions) { o.enableHeartbeat = enable }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
