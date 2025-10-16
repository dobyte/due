package kcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultClientDialAddr          = "127.0.0.1:3553"
	defaultClientDialTimeout       = "5s"
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientDialAddrKey          = "etc.network.kcp.client.addr"
	defaultClientDialTimeoutKey       = "etc.network.kcp.client.timeout"
	defaultClientHeartbeatIntervalKey = "etc.network.kcp.client.heartbeatInterval"
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
	timeout           time.Duration // 拨号超时时间，默认5s
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
	mtu               int           // 最大传输单元，默认不设置
	noDelay           []int         // 是否开启无延迟模式，默认不设置
	ackNoDelay        bool          // 是否开启ACK延迟确认，默认不设置
	writeDelay        bool          // 是否开启写延迟，默认不设置
	windowSize        []int         // 窗口大小，默认不设置
	readBuffer        int           // 读取缓冲区大小，默认不设置
	writeBuffer       int           // 写入缓冲区大小，默认不设置
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		addr:              etc.Get(defaultClientDialAddrKey, defaultClientDialAddr).String(),
		timeout:           etc.Get(defaultClientDialTimeoutKey, defaultClientDialTimeout).Duration(),
		heartbeatInterval: etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(),
		mtu:               etc.Get(defaultClientMtuKey).Int(),
		noDelay:           etc.Get(defaultClientNoDelayKey).Ints(),
		ackNoDelay:        etc.Get(defaultClientAckNoDelayKey).Bool(),
		writeDelay:        etc.Get(defaultClientWriteDelayKey).Bool(),
		windowSize:        etc.Get(defaultClientWindowSizeKey).Ints(),
		readBuffer:        int(etc.Get(defaultClientReadBufferKey).B()),
		writeBuffer:       int(etc.Get(defaultClientWriteBufferKey).B()),
	}
}

// WithClientDialAddr 设置拨号地址
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientDialTimeout 设置拨号超时时间
func WithClientDialTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.timeout = timeout }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}

// WithClientMtu 设置最大传输单元
func WithClientMtu(mtu int) ClientOption {
	return func(o *clientOptions) { o.mtu = mtu }
}

// WithClientNoDelay 设置是否开启无延迟模式
func WithClientNoDelay(noDelay int) ClientOption {
	return func(o *clientOptions) { o.noDelay = append(o.noDelay, noDelay) }
}

// WithClientAckNoDelay 设置是否开启ACK延迟确认
func WithClientAckNoDelay(ackNoDelay bool) ClientOption {
	return func(o *clientOptions) { o.ackNoDelay = ackNoDelay }
}

// WithClientWriteDelay 设置是否开启写延迟
func WithClientWriteDelay(writeDelay bool) ClientOption {
	return func(o *clientOptions) { o.writeDelay = writeDelay }
}

// WithClientWindowSize 设置窗口大小
func WithClientWindowSize(windowSize int) ClientOption {
	return func(o *clientOptions) { o.windowSize = append(o.windowSize, windowSize) }
}

// WithClientReadBuffer 设置读取缓冲区大小
func WithClientReadBuffer(readBuffer int) ClientOption {
	return func(o *clientOptions) { o.readBuffer = readBuffer }
}

// WithClientWriteBuffer 设置写入缓冲区大小
func WithClientWriteBuffer(writeBuffer int) ClientOption {
	return func(o *clientOptions) { o.writeBuffer = writeBuffer }
}
