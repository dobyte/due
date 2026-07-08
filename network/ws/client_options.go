package ws

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientUrl               = "ws://127.0.0.1:3553"
	defaultClientDialTimeout       = "3s"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientUrlKey               = "etc.network.ws.client.url"
	defaultClientDialTimeoutKey       = "etc.network.ws.client.dialTimeout"
	defaultClientWriteTimeoutKey      = "etc.network.ws.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.ws.client.writeQueueSize"
	defaultClientHeartbeatIntervalKey = "etc.network.ws.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	url               string        // 拨号地址
	dialTimeout       time.Duration // 拨号超时时间，默认3s
	writeTimeout      time.Duration // 写入超时时间，默认无超时
	writeQueueSize    int           // 写入队列大小，默认1024
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	opts := &clientOptions{}

	if url := etc.Get(defaultClientUrlKey, defaultClientUrl).String(); url != "" {
		opts.url = url
	} else {
		opts.url = defaultClientUrl
	}

	if dialTimeout := etc.Get(defaultClientDialTimeoutKey, defaultClientDialTimeout).Duration(); dialTimeout > 0 {
		opts.dialTimeout = dialTimeout
	} else {
		opts.dialTimeout = xconv.Duration(defaultClientDialTimeout)
	}

	if writeTimeout := etc.Get(defaultClientWriteTimeoutKey, defaultClientWriteTimeout).Duration(); writeTimeout >= 0 {
		opts.writeTimeout = writeTimeout
	} else {
		opts.writeTimeout = xconv.Duration(defaultClientWriteTimeout)
	}

	if writeQueueSize := etc.Get(defaultClientWriteQueueSizeKey, defaultClientWriteQueueSize).Int(); writeQueueSize > 0 {
		opts.writeQueueSize = writeQueueSize
	} else {
		opts.writeQueueSize = defaultClientWriteQueueSize
	}

	if heartbeatInterval := etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(); heartbeatInterval >= 0 {
		opts.heartbeatInterval = heartbeatInterval
	} else {
		opts.heartbeatInterval = xconv.Duration(defaultClientHeartbeatInterval)
	}

	return opts
}

// WithClientUrl 设置拨号链接
func WithClientUrl(url string) ClientOption {
	return func(o *clientOptions) {
		if url != "" {
			o.url = url
		} else {
			log.Warnf("the specified url is empty and will be ignored")
		}
	}
}

// WithClientDialTimeout 设置拨号超时时间
func WithClientDialTimeout(dialTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if dialTimeout >= 0 {
			o.dialTimeout = dialTimeout
		} else {
			log.Warnf("the specified dialTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientWriteTimeout 设置写超时时间
func WithClientWriteTimeout(writeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientWriteQueueSize 设置写队列大小
func WithClientWriteQueueSize(writeQueueSize int) ClientOption {
	return func(o *clientOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}
