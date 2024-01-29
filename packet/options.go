package packet

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/etc"
	"strings"
)

// heartbeat packet
// ------------------------------------------------------------------------------
// | size(4 byte) = (1 byte + 8 byte) | header(1 byte) | heartbeat time(8 byte) |
// ------------------------------------------------------------------------------

// data packet
// -----------------------------------------------------------------------------------------------------------------------
// | size(4 byte) = (1 byte + n byte + m byte + x byte) | header(1 byte) | route(n byte) | seq(m byte) | message(x byte) |
// -----------------------------------------------------------------------------------------------------------------------

const (
	littleEndian = "little"
	bigEndian    = "big"
)

const (
	defaultSizeBytes          = 4
	defaultHeaderBytes        = 1
	defaultRouteBytes         = 2
	defaultSeqBytes           = 2
	defaultBufferBytes        = 5000
	defaultHeartbeatTime      = false
	defaultHeartbeatTimeBytes = 8
)

const (
	defaultEndianKey        = "etc.packet.byteOrder"
	defaultRouteBytesKey    = "etc.packet.routeBytes"
	defaultSeqBytesKey      = "etc.packet.seqBytes"
	defaultBufferBytesKey   = "etc.packet.bufferBytes"
	defaultHeartbeatTimeKey = "etc.packet.heartbeatTime"
)

type options struct {
	// 字节序
	// 默认为binary.LittleEndian
	byteOrder binary.ByteOrder

	// 路由字节数
	// 默认为2字节
	routeBytes int

	// 序列号字节数，长度为0时不开启序列号编码
	// 默认为2字节
	seqBytes int

	// 消息字节数
	// 默认为5000字节
	bufferBytes int

	// 是否携带心跳时间
	// 默认为false
	heartbeatTime bool
}

type Option func(o *options)

func defaultOptions() *options {
	opts := &options{
		byteOrder:     binary.BigEndian,
		routeBytes:    etc.Get(defaultRouteBytesKey, defaultRouteBytes).Int(),
		seqBytes:      etc.Get(defaultSeqBytesKey, defaultSeqBytes).Int(),
		bufferBytes:   etc.Get(defaultBufferBytesKey, defaultBufferBytes).Int(),
		heartbeatTime: etc.Get(defaultHeartbeatTimeKey, defaultHeartbeatTime).Bool(),
	}

	endian := etc.Get(defaultEndianKey, bigEndian).String()
	switch strings.ToLower(endian) {
	case littleEndian:
		opts.byteOrder = binary.LittleEndian
	case bigEndian:
		opts.byteOrder = binary.BigEndian
	}

	return opts
}

// WithByteOrder 设置字节序
func WithByteOrder(byteOrder binary.ByteOrder) Option {
	return func(o *options) { o.byteOrder = byteOrder }
}

// WithRouteBytes 设置路由字节数
func WithRouteBytes(routeBytes int) Option {
	return func(o *options) { o.routeBytes = routeBytes }
}

// WithSeqBytes 设置序列号字节数
func WithSeqBytes(seqBytes int) Option {
	return func(o *options) { o.seqBytes = seqBytes }
}

// WithBufferBytes 设置消息字节数
func WithBufferBytes(bufferBytes int) Option {
	return func(o *options) { o.bufferBytes = bufferBytes }
}

// WithHeartbeatTime 是否携带心跳时间
func WithHeartbeatTime(heartbeatTime bool) Option {
	return func(o *options) { o.heartbeatTime = heartbeatTime }
}
