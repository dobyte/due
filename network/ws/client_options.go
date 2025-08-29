package ws

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultClientUrl               = "ws://127.0.0.1:3553"
	defaultClientHandshakeTimeout  = "10s"
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientUrlKey               = "etc.network.ws.client.url"
	defaultClientHandshakeTimeoutKey  = "etc.network.ws.client.handshakeTimeout"
	defaultClientHeartbeatIntervalKey = "etc.network.ws.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	url               string        // 拨号地址
	handshakeTimeout  time.Duration // 握手超时时间
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		url:               etc.Get(defaultClientUrlKey, defaultClientUrl).String(),
		handshakeTimeout:  etc.Get(defaultClientHandshakeTimeoutKey, defaultClientHandshakeTimeout).Duration(),
		heartbeatInterval: etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(),
	}
}

// WithClientUrl 设置拨号链接
func WithClientUrl(url string) ClientOption {
	return func(o *clientOptions) { o.url = url }
}

// WithClientHandshakeTimeout 设置握手超时时间
func WithClientHandshakeTimeout(handshakeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.handshakeTimeout = handshakeTimeout }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
