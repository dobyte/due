package kcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientDialAddr          = "127.0.0.1:3553"
	defaultClientDialTimeout       = "3s"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
	defaultClientCloseTimeout      = "0s"
	defaultClientMtu               = 1400
	defaultClientWriteDelay        = true
)

var (
	defaultClientNoDelay    = []int{1, 10, 2, 1}
	defaultClientWindowSize = []int{32, 32}
)

const (
	defaultClientDialAddrKey          = "etc.network.kcp.client.addr"
	defaultClientDialTimeoutKey       = "etc.network.kcp.client.dialTimeout"
	defaultClientDialTimeoutLegacyKey = "etc.network.kcp.client.timeout"
	defaultClientHeartbeatIntervalKey = "etc.network.kcp.client.heartbeatInterval"
	defaultClientWriteTimeoutKey      = "etc.network.kcp.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.kcp.client.writeQueueSize"
	defaultClientCloseTimeoutKey      = "etc.network.kcp.client.closeTimeout"
	defaultClientMtuKey               = "etc.network.kcp.client.mtu"
	defaultClientNoDelayKey           = "etc.network.kcp.client.noDelay"
	defaultClientAckNoDelayKey        = "etc.network.kcp.client.ackNoDelay"
	defaultClientWriteDelayKey        = "etc.network.kcp.client.writeDelay"
	defaultClientWindowSizeKey        = "etc.network.kcp.client.windowSize"
	defaultClientReadBufferKey        = "etc.network.kcp.client.readBuffer"
	defaultClientWriteBufferKey       = "etc.network.kcp.client.writeBuffer"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	dialTimeout       time.Duration // 拨号超时时间，默认3s
	writeTimeout      time.Duration // 写入超时时间，默认无超时
	writeQueueSize    int           // 写入队列大小，默认1024
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
	closeTimeout      time.Duration // 优雅关闭超时时间，默认0s，不限制
	mtu               int           // 最大传输单元，默认不设置
	noDelay           []int         // 是否开启无延迟模式，默认不设置
	ackNoDelay        bool          // 是否开启ACK延迟确认，默认不设置
	writeDelay        bool          // 是否开启写延迟，默认不设置
	windowSize        []int         // 窗口大小，默认不设置
	readBuffer        int           // 读取缓冲区大小，默认不设置
	writeBuffer       int           // 写入缓冲区大小，默认不设置
}

// defaultClientOptions 默认客户端配置
// 从配置中心读取各配置项，生成默认客户端配置
// @return @1 *clientOptions 客户端配置
func defaultClientOptions() *clientOptions {
	opts := &clientOptions{}

	if addr := etc.Get(defaultClientDialAddrKey, defaultClientDialAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultClientDialAddr
	}

	// 优先读取对齐TCP命名的新键dialTimeout，缺省时回退到历史键timeout以保持兼容
	if dialTimeout := etc.Get(defaultClientDialTimeoutKey, etc.Get(defaultClientDialTimeoutLegacyKey, defaultClientDialTimeout)).Duration(); dialTimeout > 0 {
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

	if closeTimeout := etc.Get(defaultClientCloseTimeoutKey, defaultClientCloseTimeout).Duration(); closeTimeout >= 0 {
		opts.closeTimeout = closeTimeout
	} else {
		opts.closeTimeout = xconv.Duration(defaultClientCloseTimeout)
	}

	opts.mtu = etc.Get(defaultClientMtuKey, defaultClientMtu).Int()
	opts.noDelay = etc.Get(defaultClientNoDelayKey, defaultClientNoDelay).Ints()
	opts.ackNoDelay = etc.Get(defaultClientAckNoDelayKey).Bool()
	opts.writeDelay = etc.Get(defaultClientWriteDelayKey, defaultClientWriteDelay).Bool()
	opts.windowSize = etc.Get(defaultClientWindowSizeKey, defaultClientWindowSize).Ints()
	opts.readBuffer = int(etc.Get(defaultClientReadBufferKey).B())
	opts.writeBuffer = int(etc.Get(defaultClientWriteBufferKey).B())

	return opts
}

// WithClientDialAddr 设置拨号地址
// @param addr string 拨号地址
// @return @1 ClientOption 客户端配置选项
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithClientDialTimeout 设置拨号超时时间
// @param dialTimeout time.Duration 拨号超时时间，小于0时忽略
// @return @1 ClientOption 客户端配置选项
func WithClientDialTimeout(dialTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if dialTimeout >= 0 {
			o.dialTimeout = dialTimeout
		} else {
			log.Warnf("the specified dialTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientHeartbeatInterval 设置心跳间隔时间
// @param heartbeatInterval time.Duration 心跳间隔时间
// @return @1 ClientOption 客户端配置选项
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}

// WithClientMtu 设置最大传输单元
// @param mtu int 最大传输单元
// @return @1 ClientOption 客户端配置选项
func WithClientMtu(mtu int) ClientOption {
	return func(o *clientOptions) { o.mtu = mtu }
}

// WithClientNoDelay 设置是否开启无延迟模式
// @param noDelay []int 无延迟模式参数，须为4元组(nodelay, interval, resend, nc)
// @return @1 ClientOption 客户端配置选项
func WithClientNoDelay(noDelay []int) ClientOption {
	return func(o *clientOptions) {
		if len(noDelay) == 4 {
			o.noDelay = noDelay
		} else {
			log.Warnf("the specified noDelay must be a 4-tuple and will be ignored")
		}
	}
}

// WithClientAckNoDelay 设置是否开启ACK延迟确认
// @param ackNoDelay bool 是否开启ACK延迟确认
// @return @1 ClientOption 客户端配置选项
func WithClientAckNoDelay(ackNoDelay bool) ClientOption {
	return func(o *clientOptions) { o.ackNoDelay = ackNoDelay }
}

// WithClientWriteDelay 设置是否开启写延迟
// @param writeDelay bool 是否开启写延迟
// @return @1 ClientOption 客户端配置选项
func WithClientWriteDelay(writeDelay bool) ClientOption {
	return func(o *clientOptions) { o.writeDelay = writeDelay }
}

// WithClientWindowSize 设置窗口大小
// @param windowSize []int 窗口大小取值，须为2元组(sndwnd, rcvwnd)
// @return @1 ClientOption 客户端配置选项
func WithClientWindowSize(windowSize []int) ClientOption {
	return func(o *clientOptions) {
		if len(windowSize) == 2 {
			o.windowSize = windowSize
		} else {
			log.Warnf("the specified windowSize must be a 2-tuple and will be ignored")
		}
	}
}

// WithClientReadBuffer 设置读取缓冲区大小
// @param readBuffer int 读取缓冲区大小
// @return @1 ClientOption 客户端配置选项
func WithClientReadBuffer(readBuffer int) ClientOption {
	return func(o *clientOptions) { o.readBuffer = readBuffer }
}

// WithClientWriteBuffer 设置写入缓冲区大小
// @param writeBuffer int 写入缓冲区大小
// @return @1 ClientOption 客户端配置选项
func WithClientWriteBuffer(writeBuffer int) ClientOption {
	return func(o *clientOptions) { o.writeBuffer = writeBuffer }
}

// WithClientWriteTimeout 设置写超时时间
// @param writeTimeout time.Duration 写超时时间，小于0时忽略
// @return @1 ClientOption 客户端配置选项
func WithClientWriteTimeout(writeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientWriteQueueSize 设置写入队列大小
// @param writeQueueSize int 写入队列大小，小于等于0时忽略
// @return @1 ClientOption 客户端配置选项
func WithClientWriteQueueSize(writeQueueSize int) ClientOption {
	return func(o *clientOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}

// WithClientCloseTimeout 设置优雅关闭超时时间
// 超时后未排空的写队列将放弃等待并强制关闭连接，默认为0表示不限制
// @param closeTimeout time.Duration 优雅关闭超时时间
// @return @1 ClientOption 客户端配置选项
func WithClientCloseTimeout(closeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if closeTimeout >= 0 {
			o.closeTimeout = closeTimeout
		} else {
			log.Warnf("the specified closeTimeout is less than zero and will be ignored")
		}
	}
}
