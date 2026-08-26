/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/12 12:57 上午
 * @Desc: TODO
 */

package kcp

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
	defaultServerMtu                = 1400
)

var (
	defaultServerNoDelay = []int{1, 10, 2, 1}
)

const (
	defaultServerAddrKey               = "etc.network.kcp.server.addr"
	defaultServerMaxConnNumKey         = "etc.network.kcp.server.maxConnNum"
	defaultServerWriteTimeoutKey       = "etc.network.kcp.server.writeTimeout"
	defaultServerWriteQueueSizeKey     = "etc.network.kcp.server.writeQueueSize"
	defaultServerHeartbeatIntervalKey  = "etc.network.kcp.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey = "etc.network.kcp.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey   = "etc.network.kcp.server.authorizeTimeout"
	defaultServerMtuKey                = "etc.network.kcp.server.mtu"
	defaultServerNoDelayKey            = "etc.network.kcp.server.noDelay"
	defaultServerAckNoDelayKey         = "etc.network.kcp.server.ackNoDelay"
	defaultServerWriteDelayKey         = "etc.network.kcp.server.writeDelay"
	defaultServerWindowSizeKey         = "etc.network.kcp.server.windowSize"
	defaultServerReadBufferKey         = "etc.network.kcp.server.readBuffer"
	defaultServerWriteBufferKey        = "etc.network.kcp.server.writeBuffer"
)

const (
	RespHeartbeat HeartbeatMechanism = "resp" // 响应式心跳
	TickHeartbeat HeartbeatMechanism = "tick" // 主动定时心跳
)

type HeartbeatMechanism string

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr               string             // 监听地址
	maxConnNum         int                // 最大连接数
	writeTimeout       time.Duration      // 写入超时时间，默认无超时
	writeQueueSize     int                // 写入队列大小，默认1024
	heartbeatInterval  time.Duration      // 心跳检测间隔时间，默认10s
	heartbeatMechanism HeartbeatMechanism // 心跳机制，默认resp
	authorizeTimeout   time.Duration      // 授权超时时间，默认0s，不检测
	mtu                int                // 最大传输单元，默认不设置
	noDelay            []int              // 是否开启无延迟模式，默认不设置
	ackNoDelay         bool               // 是否开启ACK延迟确认，默认不设置
	writeDelay         bool               // 是否开启写延迟，默认不设置
	windowSize         []int              // 窗口大小，默认不设置
	readBuffer         int                // 读取缓冲区大小，默认不设置
	writeBuffer        int                // 写入缓冲区大小，默认不设置
}

// defaultServerOptions 默认服务器配置
// 从配置中心读取各配置项，生成默认服务器配置
// @return @1 *serverOptions 服务器配置
func defaultServerOptions() *serverOptions {
	opts := &serverOptions{}
	opts.addr = etc.Get(defaultServerAddrKey, defaultServerAddr).String()
	opts.maxConnNum = etc.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int()

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

	opts.heartbeatInterval = etc.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration()
	opts.heartbeatMechanism = HeartbeatMechanism(etc.Get(defaultServerHeartbeatMechanismKey, defaultServerHeartbeatMechanism).String())
	opts.authorizeTimeout = etc.Get(defaultServerAuthorizeTimeoutKey, defaultServerAuthorizeTimeout).Duration()
	opts.mtu = etc.Get(defaultServerMtuKey, defaultServerMtu).Int()
	opts.noDelay = etc.Get(defaultServerNoDelayKey, defaultServerNoDelay).Ints()
	opts.ackNoDelay = etc.Get(defaultServerAckNoDelayKey).Bool()
	opts.writeDelay = etc.Get(defaultServerWriteDelayKey).Bool()
	opts.windowSize = etc.Get(defaultServerWindowSizeKey).Ints()
	opts.readBuffer = int(etc.Get(defaultServerReadBufferKey).B())
	opts.writeBuffer = int(etc.Get(defaultServerWriteBufferKey).B())

	return opts
}

// WithServerListenAddr 设置监听地址
// @param addr string 监听地址
// @return @1 ServerOption 服务器配置选项
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) { o.addr = addr }
}

// WithServerMaxConnNum 设置连接的最大连接数
// @param maxConnNum int 最大连接数
// @return @1 ServerOption 服务器配置选项
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) { o.maxConnNum = maxConnNum }
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
// @param heartbeatInterval time.Duration 心跳检测间隔时间
// @return @1 ServerOption 服务器配置选项
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatInterval = heartbeatInterval }
}

// WithServerHeartbeatMechanism 设置心跳机制
// @param heartbeatMechanism HeartbeatMechanism 心跳机制
// @return @1 ServerOption 服务器配置选项
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) { o.heartbeatMechanism = heartbeatMechanism }
}

// WithServerAuthorizeTimeout 设置授权超时时间
// @param authorizeTimeout time.Duration 授权超时时间
// @return @1 ServerOption 服务器配置选项
func WithServerAuthorizeTimeout(authorizeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) { o.authorizeTimeout = authorizeTimeout }
}

// WithServerMtu 设置最大传输单元
// @param mtu int 最大传输单元
// @return @1 ServerOption 服务器配置选项
func WithServerMtu(mtu int) ServerOption {
	return func(o *serverOptions) { o.mtu = mtu }
}

// WithServerNoDelay 设置是否开启无延迟模式
// @param noDelay []int 是否开启无延迟模式的取值
// @return @1 ServerOption 服务器配置选项
func WithServerNoDelay(noDelay []int) ServerOption {
	return func(o *serverOptions) { o.noDelay = noDelay }
}

// WithServerAckNoDelay 设置是否开启ACK延迟确认
// @param ackNoDelay bool 是否开启ACK延迟确认
// @return @1 ServerOption 服务器配置选项
func WithServerAckNoDelay(ackNoDelay bool) ServerOption {
	return func(o *serverOptions) { o.ackNoDelay = ackNoDelay }
}

// WithServerWriteDelay 设置是否开启写延迟
// @param writeDelay bool 是否开启写延迟
// @return @1 ServerOption 服务器配置选项
func WithServerWriteDelay(writeDelay bool) ServerOption {
	return func(o *serverOptions) { o.writeDelay = writeDelay }
}

// WithServerWindowSize 设置窗口大小
// @param windowSize []int 窗口大小取值
// @return @1 ServerOption 服务器配置选项
func WithServerWindowSize(windowSize []int) ServerOption {
	return func(o *serverOptions) { o.windowSize = windowSize }
}

// WithServerReadBuffer 设置读取缓冲区大小
// @param readBuffer int 读取缓冲区大小
// @return @1 ServerOption 服务器配置选项
func WithServerReadBuffer(readBuffer int) ServerOption {
	return func(o *serverOptions) { o.readBuffer = readBuffer }
}

// WithServerWriteBuffer 设置写入缓冲区大小
// @param writeBuffer int 写入缓冲区大小
// @return @1 ServerOption 服务器配置选项
func WithServerWriteBuffer(writeBuffer int) ServerOption {
	return func(o *serverOptions) { o.writeBuffer = writeBuffer }
}

// WithServerWriteTimeout 设置写超时时间
// @param writeTimeout time.Duration 写超时时间，小于0时忽略
// @return @1 ServerOption 服务器配置选项
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
// @param writeQueueSize int 写入队列大小，小于等于0时忽略
// @return @1 ServerOption 服务器配置选项
func WithServerWriteQueueSize(writeQueueSize int) ServerOption {
	return func(o *serverOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}
