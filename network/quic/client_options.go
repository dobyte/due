package quic

import (
	"crypto/tls"
	"time"

	ctls "github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientAddr              = "127.0.0.1:3553"
	defaultClientDialTimeout       = "5s"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientAddrKey              = "etc.network.quic.client.addr"
	defaultClientCAFileKey            = "etc.network.quic.client.caFile"
	defaultClientServerNameKey        = "etc.network.quic.client.serverName"
	defaultClientDialTimeoutKey       = "etc.network.quic.client.dialTimeout"
	defaultClientWriteTimeoutKey      = "etc.network.quic.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.quic.client.writeQueueSize"
	defaultClientHeartbeatIntervalKey = "etc.network.quic.client.heartbeatInterval"
)

// ClientOption 客户端配置项
// @param o *clientOptions 客户端配置
type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	tlsConfig         *tls.Config   // TLS配置
	dialTimeout       time.Duration // 拨号超时时间，默认5s
	writeTimeout      time.Duration // 写超时时间，默认无超时
	writeQueueSize    int           // 写队列大小，默认1024
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

// defaultClientOptions 构建默认客户端配置
// 优先读取环境配置（etc.network.quic.client.*），缺失时回退到内置默认值，并尝试加载CA证书构建TLS配置
// @return @1 *clientOptions 客户端配置
func defaultClientOptions() *clientOptions {
	opts := &clientOptions{}

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

	caFile := etc.Get(defaultClientCAFileKey).String()
	serverName := etc.Get(defaultClientServerNameKey).String()

	if caFile != "" || serverName != "" {
		if config, err := ctls.MakeTCPClientTLSConfig(caFile, serverName); err != nil {
			log.Warnf("make tcp client tls config failed: %v", err)
		} else {
			opts.tlsConfig = config
		}
	}

	return opts
}

// WithClientAddr 设置拨号地址
// @param addr string 连接地址，为空时忽略
// @return @1 ClientOption 客户端配置项
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
// @param caFile string CA证书文件
// @param serverName string 服务器名称
// @return @1 ClientOption 客户端配置项
func WithClientCredentials(caFile string, serverName string) ClientOption {
	return func(o *clientOptions) {
		if caFile != "" || serverName != "" {
			if config, err := ctls.MakeTCPClientTLSConfig(caFile, serverName); err != nil {
				log.Warnf("make tcp client tls config failed: %v", err)
			} else {
				o.tlsConfig = config
			}
		} else {
			log.Warnf("the specified caFile or serverName is empty and will be ignored")
		}
	}
}

// WithClientTLSConfig 设置TLS配置
// @param tlsConfig *tls.Config TLS配置
// @return @1 ClientOption 客户端配置项
func WithClientTLSConfig(tlsConfig *tls.Config) ClientOption {
	return func(o *clientOptions) {
		o.tlsConfig = tlsConfig
	}
}

// WithClientDialTimeout 设置拨号超时时间
// @param dialTimeout time.Duration 拨号超时时间，小于0时忽略
// @return @1 ClientOption 客户端配置项
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
// @param writeTimeout time.Duration 写超时时间，小于0时忽略
// @return @1 ClientOption 客户端配置项
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
// @param writeQueueSize int 写队列大小，小于等于0时忽略
// @return @1 ClientOption 客户端配置项
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
// @param heartbeatInterval time.Duration 心跳间隔时间，小于0时忽略
// @return @1 ClientOption 客户端配置项
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}
