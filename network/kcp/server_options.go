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
)

const (
	defaultServerAddr               = ":3553"
	defaultServerMaxConnNum         = 5000
	defaultServerHeartbeatInterval  = "10s"
	defaultServerHeartbeatMechanism = "resp"
	defaultServerAuthorizeTimeout   = "0s"
)

const (
	defaultServerAddrKey               = "etc.network.kcp.server.addr"
	defaultServerMaxConnNumKey         = "etc.network.kcp.server.maxConnNum"
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

func defaultServerOptions() *serverOptions {
	return &serverOptions{
		addr:               etc.Get(defaultServerAddrKey, defaultServerAddr).String(),
		maxConnNum:         etc.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(),
		heartbeatInterval:  etc.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration(),
		heartbeatMechanism: HeartbeatMechanism(etc.Get(defaultServerHeartbeatMechanismKey, defaultServerHeartbeatMechanism).String()),
		authorizeTimeout:   etc.Get(defaultServerAuthorizeTimeoutKey, defaultServerAuthorizeTimeout).Duration(),
		mtu:                etc.Get(defaultServerMtuKey).Int(),
		noDelay:            etc.Get(defaultServerNoDelayKey).Ints(),
		ackNoDelay:         etc.Get(defaultServerAckNoDelayKey).Bool(),
		writeDelay:         etc.Get(defaultServerWriteDelayKey).Bool(),
		windowSize:         etc.Get(defaultServerWindowSizeKey).Ints(),
		readBuffer:         int(etc.Get(defaultServerReadBufferKey).B()),
		writeBuffer:        int(etc.Get(defaultServerWriteBufferKey).B()),
	}
}

// WithServerListenAddr 设置监听地址
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) { o.addr = addr }
}

// WithServerMaxConnNum 设置连接的最大连接数
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) { o.maxConnNum = maxConnNum }
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatInterval = heartbeatInterval }
}

// WithServerHeartbeatMechanism 设置心跳机制
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) { o.heartbeatMechanism = heartbeatMechanism }
}

// WithServerAuthorizeTimeout 设置授权超时时间
func WithServerAuthorizeTimeout(authorizeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) { o.authorizeTimeout = authorizeTimeout }
}

// WithServerMtu 设置最大传输单元
func WithServerMtu(mtu int) ServerOption {
	return func(o *serverOptions) { o.mtu = mtu }
}

// WithServerNoDelay 设置是否开启无延迟模式
func WithServerNoDelay(noDelay []int) ServerOption {
	return func(o *serverOptions) { o.noDelay = noDelay }
}

// WithServerAckNoDelay 设置是否开启ACK延迟确认
func WithServerAckNoDelay(ackNoDelay bool) ServerOption {
	return func(o *serverOptions) { o.ackNoDelay = ackNoDelay }
}

// WithServerWriteDelay 设置是否开启写延迟
func WithServerWriteDelay(writeDelay bool) ServerOption {
	return func(o *serverOptions) { o.writeDelay = writeDelay }
}

// WithServerWindowSize 设置窗口大小
func WithServerWindowSize(windowSize []int) ServerOption {
	return func(o *serverOptions) { o.windowSize = windowSize }
}

// WithServerReadBuffer 设置读取缓冲区大小
func WithServerReadBuffer(readBuffer int) ServerOption {
	return func(o *serverOptions) { o.readBuffer = readBuffer }
}

// WithServerWriteBuffer 设置写入缓冲区大小
func WithServerWriteBuffer(writeBuffer int) ServerOption {
	return func(o *serverOptions) { o.writeBuffer = writeBuffer }
}
