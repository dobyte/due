package ws

import "time"

type ClientOption func(o *clientOptions)

type clientOptions struct {
	url               string        // 地址
	maxMsgLength      int           // 最大消息长度
	handshakeTimeout  time.Duration // 握手超时时间
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

// WithClientDialUrl 设置拨号链接
func WithClientDialUrl(url string) ClientOption {
	return func(o *clientOptions) { o.url = url }
}

// WithClientMaxMsgLength 设置消息最大长度
func WithClientMaxMsgLength(maxMsgLength int) ClientOption {
	return func(o *clientOptions) { o.maxMsgLength = maxMsgLength }
}

// WithClientHandshakeTimeout 设置握手超时时间
func WithClientHandshakeTimeout(handshakeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.handshakeTimeout = handshakeTimeout }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
