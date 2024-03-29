package packet

import (
	"encoding/binary"
	"github.com/symsimmy/due/config"
	"strings"
)

const (
	littleEndian = "little"
	bigEndian    = "big"
)

const (
	defaultCompressBytesLen  = 1
	defaultSeqBytesLen       = 2
	defaultRouteBytesLen     = 2
	defaultBufferBytesLen    = 5000
	defaultCompressEnable    = false
	defaultCompressAlgorithm = "snappy"
	defaultThreshold         = 1024
)

const (
	defaultEndianKey            = "config.packet.endian"
	defaultSeqBytesLenKey       = "config.packet.seqBytesLen"
	defaultRouteBytesLenKey     = "config.packet.routeBytesLen"
	defaultBufferBytesLenKey    = "config.packet.bufferBytesLen"
	defaultCompressBytesLenKey  = "config.packet.compressBytesLen"
	defaultCompressEnableKey    = "config.packet.compress.enable"
	defaultCompressAlgorithmKey = "config.packet.compress.algorithm"
	defaultCompressThresholdKey = "config.packet.compress.threshold"
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

	// 消息字节长度（字节）
	// 默认为5000字节
	bufferBytesLen int

	// 消息是否压缩字节长度（字节）
	// 默认为1字节
	compressBytesLen int

	// 消息是否启用压缩算法
	//默认为不启用，false
	compressEnable bool

	// 消息使用何种压缩算法
	//默认为 snappy 算法，还有 klugzip 待选
	compressAlgorithm string

	// 消息压缩的启用阈值，大于这个阈值的消息均压缩，小于则不压缩
	//默认为1024字节
	compressThreshold int
}

type Option func(o *options)

func defaultOptions() *options {
	opts := &options{
		byteOrder:         binary.LittleEndian,
		seqBytesLen:       config.Get(defaultSeqBytesLenKey, defaultSeqBytesLen).Int(),
		routeBytesLen:     config.Get(defaultRouteBytesLenKey, defaultRouteBytesLen).Int(),
		bufferBytesLen:    config.Get(defaultBufferBytesLenKey, defaultBufferBytesLen).Int(),
		compressBytesLen:  config.Get(defaultCompressBytesLenKey, defaultCompressBytesLen).Int(),
		compressEnable:    config.Get(defaultCompressEnableKey, defaultCompressEnable).Bool(),
		compressAlgorithm: config.Get(defaultCompressAlgorithmKey, defaultCompressAlgorithm).String(),
		compressThreshold: config.Get(defaultCompressThresholdKey, defaultThreshold).Int(),
	}

	endian := config.Get(defaultEndianKey).String()
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

// WithSeqBytesLen 设置序列号字节长度
func WithSeqBytesLen(seqBytesLen int) Option {
	return func(o *options) { o.seqBytesLen = seqBytesLen }
}

// WithRouteBytesLen 设置路由字节长度
func WithRouteBytesLen(routeBytesLen int) Option {
	return func(o *options) { o.routeBytesLen = routeBytesLen }
}

// WithBufferBytesLen 设置消息字节长度
func WithBufferBytesLen(bufferBytesLen int) Option {
	return func(o *options) { o.bufferBytesLen = bufferBytesLen }
}

// WithCompressBytesLen 设置消息是否压缩标志位字节长度
func WithCompressBytesLen(compressBytesLen int) Option {
	return func(o *options) { o.compressBytesLen = compressBytesLen }
}
