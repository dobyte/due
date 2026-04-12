package ws

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultClientUrl               = "ws://127.0.0.1:3553"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHandshakeTimeout  = "10s"
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientUrlKey               = "etc.network.ws.client.url"
	defaultClientWriteTimeoutKey      = "etc.network.ws.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.ws.client.writeQueueSize"
	defaultClientHandshakeTimeoutKey  = "etc.network.ws.client.handshakeTimeout"
	defaultClientHeartbeatIntervalKey = "etc.network.ws.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	url               string        // 拨号地址
	writeTimeout      time.Duration // 写入超时时间，默认无超时
	writeQueueSize    int           // 写入队列大小，默认1024
	handshakeTimeout  time.Duration // 握手超时时间，默认10s
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	writeQueueSize := etc.Get(defaultClientWriteQueueSizeKey, defaultClientWriteQueueSize).Int()

	if writeQueueSize <= 0 {
		writeQueueSize = defaultClientWriteQueueSize
	}

	return &clientOptions{
		url:               etc.Get(defaultClientUrlKey, defaultClientUrl).String(),
		writeTimeout:      etc.Get(defaultClientWriteTimeoutKey, defaultClientWriteTimeout).Duration(),
		writeQueueSize:    writeQueueSize,
		handshakeTimeout:  etc.Get(defaultClientHandshakeTimeoutKey, defaultClientHandshakeTimeout).Duration(),
		heartbeatInterval: etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(),
	}
}

// WithClientUrl 设置拨号链接
func WithClientUrl(url string) ClientOption {
	return func(o *clientOptions) { o.url = url }
}

// WithClientWriteTimeout 设置写入超时时间
func WithClientWriteTimeout(writeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.writeTimeout = writeTimeout }
}

// WithClientWriteQueueSize 设置写入队列大小
func WithClientWriteQueueSize(writeQueueSize int) ClientOption {
	return func(o *clientOptions) {
		if writeQueueSize <= 0 {
			o.writeQueueSize = defaultClientWriteQueueSize
		} else {
			o.writeQueueSize = writeQueueSize
		}
	}
}

// WithClientHandshakeTimeout 设置握手超时时间
func WithClientHandshakeTimeout(handshakeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.handshakeTimeout = handshakeTimeout }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
