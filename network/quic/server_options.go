package quic

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultServerAddr               = ":3553"
	defaultServerMaxConnNum         = 5000
	defaultServerWriteTimeout       = "0s"
	defaultServerWriteQueueSize     = 1024
	defaultServerHeartbeatInterval  = "10s"
	defaultServerHeartbeatMechanism = "resp"
	defaultServerAuthorizeTimeout   = "0s"
	defaultServerHandshakeTimeout   = "5s"
)

const (
	defaultServerAddrKey               = "etc.network.quic.server.addr"
	defaultServerCertFileKey           = "etc.network.quic.server.certFile"
	defaultServerKeyFileKey            = "etc.network.quic.server.keyFile"
	defaultServerMaxConnNumKey         = "etc.network.quic.server.maxConnNum"
	defaultServerWriteTimeoutKey       = "etc.network.quic.server.writeTimeout"
	defaultServerWriteQueueSizeKey     = "etc.network.quic.server.writeQueueSize"
	defaultServerHeartbeatIntervalKey  = "etc.network.quic.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey = "etc.network.quic.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey   = "etc.network.quic.server.authorizeTimeout"
	defaultServerHandshakeTimeoutKey   = "etc.network.quic.server.handshakeTimeout"
)

const (
	// RespHeartbeat 响应式心跳：仅在收到对端心跳时回复心跳包
	RespHeartbeat HeartbeatMechanism = "resp"
	// TickHeartbeat 主动定时心跳：按心跳间隔主动下发心跳包
	TickHeartbeat HeartbeatMechanism = "tick"
)

// HeartbeatMechanism 心跳机制
type HeartbeatMechanism string

// ServerOption 服务器配置项
// @param o *serverOptions 服务器配置
type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr               string             // 监听地址，默认0.0.0.0:3553
	certFile           string             // 证书文件
	keyFile            string             // 秘钥文件
	maxConnNum         int                // 最大连接数，默认5000
	writeTimeout       time.Duration      // 写超时时间，默认无超时
	writeQueueSize     int                // 写队列大小，默认1024
	heartbeatInterval  time.Duration      // 心跳检测间隔时间，默认10s
	heartbeatMechanism HeartbeatMechanism // 心跳机制，默认resp
	authorizeTimeout   time.Duration      // 授权超时时间，默认0s，不检测
	handshakeTimeout   time.Duration      // 握手超时时间，默认5s
}

// defaultServerOptions 构建默认服务器配置
// 优先读取环境配置（etc.network.quic.server.*），缺失时回退到内置默认值
// @return @1 *serverOptions 服务器配置
func defaultServerOptions() *serverOptions {
	opts := &serverOptions{}
	opts.certFile = etc.Get(defaultServerCertFileKey).String()
	opts.keyFile = etc.Get(defaultServerKeyFileKey).String()

	if addr := etc.Get(defaultServerAddrKey, defaultServerAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultServerAddr
	}

	if maxConnNum := etc.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(); maxConnNum > 0 {
		opts.maxConnNum = maxConnNum
	} else {
		opts.maxConnNum = defaultServerMaxConnNum
	}

	if writeTimeout := etc.Get(defaultServerWriteTimeoutKey, defaultServerWriteTimeout).Duration(); writeTimeout >= 0 {
		opts.writeTimeout = writeTimeout
	} else {
		opts.writeTimeout = xconv.Duration(defaultServerWriteTimeout)
	}

	if writeQueueSize := etc.Get(defaultServerWriteQueueSizeKey, defaultServerWriteQueueSize).Int(); writeQueueSize > 0 {
		opts.writeQueueSize = writeQueueSize
	} else {
		opts.writeQueueSize = defaultServerWriteQueueSize
	}

	if heartbeatInterval := etc.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration(); heartbeatInterval >= 0 {
		opts.heartbeatInterval = heartbeatInterval
	} else {
		opts.heartbeatInterval = xconv.Duration(defaultServerHeartbeatInterval)
	}

	switch heartbeatMechanism := HeartbeatMechanism(etc.Get(defaultServerHeartbeatMechanismKey, defaultServerHeartbeatMechanism).String()); heartbeatMechanism {
	case RespHeartbeat, TickHeartbeat:
		opts.heartbeatMechanism = heartbeatMechanism
	default:
		opts.heartbeatMechanism = defaultServerHeartbeatMechanism
	}

	if authorizeTimeout := etc.Get(defaultServerAuthorizeTimeoutKey, defaultServerAuthorizeTimeout).Duration(); authorizeTimeout >= 0 {
		opts.authorizeTimeout = authorizeTimeout
	} else {
		opts.authorizeTimeout = xconv.Duration(defaultServerAuthorizeTimeout)
	}

	if handshakeTimeout := etc.Get(defaultServerHandshakeTimeoutKey, defaultServerHandshakeTimeout).Duration(); handshakeTimeout > 0 {
		opts.handshakeTimeout = handshakeTimeout
	} else {
		opts.handshakeTimeout = xconv.Duration(defaultServerHandshakeTimeout)
	}

	return opts
}

// WithServerAddr 设置监听地址
// @param addr string 监听地址，为空时忽略
// @return @1 ServerOption 服务器配置项
func WithServerAddr(addr string) ServerOption {
	return func(o *serverOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithServerCredentials 设置服务器证书和秘钥
// @param certFile string 证书文件
// @param keyFile string 私钥文件
// @return @1 ServerOption 服务器配置项
func WithServerCredentials(certFile, keyFile string) ServerOption {
	return func(o *serverOptions) {
		if certFile != "" && keyFile != "" {
			o.certFile, o.keyFile = certFile, keyFile
		} else {
			log.Warnf("the specified certFile or keyFile is empty and will be ignored")
		}
	}
}

// WithServerMaxConnNum 设置连接的最大连接数
// @param maxConnNum int 最大连接数，小于等于0时忽略
// @return @1 ServerOption 服务器配置项
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) {
		if maxConnNum > 0 {
			o.maxConnNum = maxConnNum
		} else {
			log.Warnf("the specified maxConnNum is less than zero and will be ignored")
		}
	}
}

// WithServerWriteTimeout 设置写超时时间
// @param writeTimeout time.Duration 写超时时间，小于0时忽略
// @return @1 ServerOption 服务器配置项
func WithServerWriteTimeout(writeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerWriteQueueSize 设置写入队列大小
// @param writeQueueSize int 写队列大小，小于等于0时忽略
// @return @1 ServerOption 服务器配置项
func WithServerWriteQueueSize(writeQueueSize int) ServerOption {
	return func(o *serverOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
// @param heartbeatInterval time.Duration 心跳间隔时间，小于0时忽略
// @return @1 ServerOption 服务器配置项
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatMechanism 设置心跳机制
// @param heartbeatMechanism HeartbeatMechanism 心跳机制，取值RespHeartbeat或TickHeartbeat
// @return @1 ServerOption 服务器配置项
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) { o.heartbeatMechanism = heartbeatMechanism }
}

// WithServerAuthorizeTimeout 设置授权超时时间
// @param authorizeTimeout time.Duration 授权超时时间，小于0时忽略，0表示不检测
// @return @1 ServerOption 服务器配置项
func WithServerAuthorizeTimeout(authorizeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if authorizeTimeout >= 0 {
			o.authorizeTimeout = authorizeTimeout
		} else {
			log.Warnf("the specified authorizeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerHandshakeTimeout 设置握手超时时间
// @param handshakeTimeout time.Duration 握手超时时间，小于等于0时忽略
// @return @1 ServerOption 服务器配置项
func WithServerHandshakeTimeout(handshakeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if handshakeTimeout > 0 {
			o.handshakeTimeout = handshakeTimeout
		} else {
			log.Warnf("the specified handshakeTimeout is less than zero and will be ignored")
		}
	}
}
