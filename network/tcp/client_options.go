package tcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientAddr              = "127.0.0.1:3553"
	defaultClientDialTimeout       = "3s"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientAddrKey              = "etc.network.tcp.client.addr"
	defaultClientCAFileKey            = "etc.network.tcp.client.caFile"
	defaultClientServerNameKey        = "etc.network.tcp.client.serverName"
	defaultClientDialTimeoutKey       = "etc.network.tcp.client.dialTimeout"
	defaultClientWriteTimeoutKey      = "etc.network.tcp.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.tcp.client.writeQueueSize"
	defaultClientHeartbeatIntervalKey = "etc.network.tcp.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	caFile            string        // CA证书文件
	serverName        string        // 服务器名称
	dialTimeout       time.Duration // 拨号超时时间，默认3s
	writeTimeout      time.Duration // 写超时时间，默认无超时
	writeQueueSize    int           // 写队列大小，默认1024
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	opts := &clientOptions{}
	opts.caFile = etc.Get(defaultClientCAFileKey).String()
	opts.serverName = etc.Get(defaultClientServerNameKey).String()

	if addr := etc.Get(defaultClientAddrKey, defaultClientAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultClientAddr
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

// WithClientAddr 设置拨号地址
func WithClientAddr(addr string) ClientOption {
	return func(o *clientOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithClientCredentials 设置CA证书和校验域名
func WithClientCredentials(caFile string, serverName string) ClientOption {
	return func(o *clientOptions) {
		if caFile != "" && serverName != "" {
			o.caFile, o.serverName = caFile, serverName
		} else {
			log.Warnf("the specified caFile or serverName is empty and will be ignored")
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
