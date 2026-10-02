package packet

import (
	"encoding/binary"
	"strings"

	"github.com/dobyte/due/v2/etc"
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
	// Byte order; defaults to binary.BigEndian.
	byteOrder binary.ByteOrder

	// Number of route bytes; defaults to 2 bytes.
	routeBytes int

	// Number of sequence-number bytes; sequence-number encoding is disabled when it is 0.
	// Defaults to 2 bytes.
	seqBytes int

	// Number of message bytes; defaults to 5000 bytes.
	bufferBytes int

	// Whether the heartbeat time is carried; defaults to false.
	heartbeatTime bool
}

type Option func(o *options)

// defaultOptions returns the default packing options.
//
// It reads each option from the configuration center to build the default packing options.
func defaultOptions() *options {
	opts := &options{
		byteOrder:     binary.BigEndian,
		routeBytes:    etc.Get(defaultRouteBytesKey, defaultRouteBytes).Int(),
		seqBytes:      etc.Get(defaultSeqBytesKey, defaultSeqBytes).Int(),
		bufferBytes:   etc.Get(defaultBufferBytesKey, defaultBufferBytes).Int(),
		heartbeatTime: etc.Get(defaultHeartbeatTimeKey, defaultHeartbeatTime).Bool(),
	}

	switch endian := etc.Get(defaultEndianKey, bigEndian).String(); strings.ToLower(endian) {
	case littleEndian:
		opts.byteOrder = binary.LittleEndian
	case bigEndian:
		opts.byteOrder = binary.BigEndian
	}

	return opts
}

// WithByteOrder returns an Option that sets the byte order.
func WithByteOrder(byteOrder binary.ByteOrder) Option {
	return func(o *options) { o.byteOrder = byteOrder }
}

// WithRouteBytes returns an Option that sets the number of route bytes.
//
// The number of route bytes must be 1, 2 or 4.
func WithRouteBytes(routeBytes int) Option {
	return func(o *options) { o.routeBytes = routeBytes }
}

// WithSeqBytes returns an Option that sets the number of sequence-number bytes.
//
// It may be 0, 1, 2 or 4; sequence-number encoding is disabled when it is 0.
func WithSeqBytes(seqBytes int) Option {
	return func(o *options) { o.seqBytes = seqBytes }
}

// WithBufferBytes returns an Option that sets the number of message bytes, that is, the maximum
// allowed message size.
func WithBufferBytes(bufferBytes int) Option {
	return func(o *options) { o.bufferBytes = bufferBytes }
}

// WithHeartbeatTime returns an Option that sets whether the heartbeat time is carried.
func WithHeartbeatTime(heartbeatTime bool) Option {
	return func(o *options) { o.heartbeatTime = heartbeatTime }
}
