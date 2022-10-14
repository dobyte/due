package packet

import (
	"encoding/binary"
	"github.com/dobyte/due/config"
	"strings"
)

const (
	littleEndian = "little"
	bigEndian    = "big"
)

const (
	defaultSeqBytesLen   = 2
	defaultRouteBytesLen = 2
)

type options struct {
	// 字节序
	// 默认为binary.LittleEndian
	byteOrder binary.ByteOrder

	// 序列号字节长度（字节），长度为0时不开启序列号编码
	// 默认为2字节，最大值为65535
	seqBytesLen int

	// 路由字节长度（字节）
	// 默认为2字节，最大值为65535
	routeBytesLen int
}

type Option func(o *options)

func defaultOptions() *options {
	opts := &options{
		byteOrder:     binary.LittleEndian,
		seqBytesLen:   defaultSeqBytesLen,
		routeBytesLen: defaultRouteBytesLen,
	}

	endian := config.Get("config.packet.endian").String()
	switch strings.ToLower(endian) {
	case littleEndian:
		opts.byteOrder = binary.LittleEndian
	case bigEndian:
		opts.byteOrder = binary.BigEndian
	}

	seqBytesLen := config.Get("config.packet.seqBytesLen").Int()
	if seqBytesLen > 0 {
		opts.seqBytesLen = seqBytesLen
	}

	routeBytesLen := config.Get("config.packet.routeBytesLen").Int()
	if routeBytesLen > 0 {
		opts.routeBytesLen = routeBytesLen
	}

	return opts
}

// WithByteOrder 设置字节序
func WithByteOrder(byteOrder binary.ByteOrder) Option {
	return func(o *options) { o.byteOrder = binary.LittleEndian }
}

// WithSeqBytesLen 设置序列号字节长度
func WithSeqBytesLen(seqBytesLen int) Option {
	return func(o *options) { o.seqBytesLen = seqBytesLen }
}

// WithRouteBytesLen 设置路由字节长度
func WithRouteBytesLen(routeBytesLen int) Option {
	return func(o *options) { o.routeBytesLen = routeBytesLen }
}
