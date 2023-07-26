package packet

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/config"
	"strings"
)

const (
	littleEndian = "little"
	bigEndian    = "big"
)

const (
	defaultRouteBytes   = 2
	defaultSeqBytes     = 2
	defaultMessageBytes = 5000
)

const (
	defaultEndianKey       = "config.packet.byteOrder"
	defaultRouteBytesKey   = "config.packet.routeBytes"
	defaultSeqBytesKey     = "config.packet.seqBytes"
	defaultMessageBytesKey = "config.packet.bufferBytes"
)

// -------------------------
// | route | seq | message |
// -------------------------
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
}

type Option func(o *options)

func defaultOptions() *options {
	opts := &options{
		byteOrder:   binary.BigEndian,
		routeBytes:  config.Get(defaultRouteBytesKey, defaultRouteBytes).Int(),
		seqBytes:    config.Get(defaultSeqBytesKey, defaultSeqBytes).Int(),
		bufferBytes: config.Get(defaultMessageBytesKey, defaultMessageBytes).Int(),
	}

	endian := config.Get(defaultEndianKey, bigEndian).String()
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

// WithMessageBytes 设置消息字节数
func WithMessageBytes(messageBytes int) Option {
	return func(o *options) { o.bufferBytes = messageBytes }
}
