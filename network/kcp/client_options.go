package kcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientDialAddr          = "127.0.0.1:3553"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
	defaultClientMtu               = 1400
)

var (
	defaultClientNoDelay = []int{1, 10, 2, 1}
)

const (
	defaultClientDialAddrKey          = "etc.network.kcp.client.addr"
	defaultClientDialTimeoutKey       = "etc.network.kcp.client.timeout"
	defaultClientHeartbeatIntervalKey = "etc.network.kcp.client.heartbeatInterval"
	defaultClientWriteTimeoutKey      = "etc.network.kcp.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.kcp.client.writeQueueSize"
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
	writeTimeout      time.Duration // 写入超时时间，默认无超时
	writeQueueSize    int           // 写入队列大小，默认1024
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
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
	opts.addr = etc.Get(defaultClientDialAddrKey, defaultClientDialAddr).String()

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

	opts.heartbeatInterval = etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration()
	opts.mtu = etc.Get(defaultClientMtuKey, defaultClientMtu).Int()
	opts.noDelay = etc.Get(defaultClientNoDelayKey, defaultClientNoDelay).Ints()
	opts.ackNoDelay = etc.Get(defaultClientAckNoDelayKey).Bool()
	opts.writeDelay = etc.Get(defaultClientWriteDelayKey).Bool()
	opts.windowSize = etc.Get(defaultClientWindowSizeKey).Ints()
	opts.readBuffer = int(etc.Get(defaultClientReadBufferKey).B())
	opts.writeBuffer = int(etc.Get(defaultClientWriteBufferKey).B())

	return opts
}

// WithClientDialAddr 设置拨号地址
// @param addr string 拨号地址
// @return @1 ClientOption 客户端配置选项
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
// @param heartbeatInterval time.Duration 心跳间隔时间
// @return @1 ClientOption 客户端配置选项
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}

// WithClientMtu 设置最大传输单元
// @param mtu int 最大传输单元
// @return @1 ClientOption 客户端配置选项
func WithClientMtu(mtu int) ClientOption {
	return func(o *clientOptions) { o.mtu = mtu }
}

// WithClientNoDelay 设置是否开启无延迟模式
// @param noDelay int 是否开启无延迟模式的取值
// @return @1 ClientOption 客户端配置选项
func WithClientNoDelay(noDelay int) ClientOption {
	return func(o *clientOptions) { o.noDelay = append(o.noDelay, noDelay) }
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
// @param windowSize int 窗口大小取值
// @return @1 ClientOption 客户端配置选项
func WithClientWindowSize(windowSize int) ClientOption {
	return func(o *clientOptions) { o.windowSize = append(o.windowSize, windowSize) }
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
