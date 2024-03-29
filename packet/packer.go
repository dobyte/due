package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/utils/compress/gzip"
	"github.com/symsimmy/due/utils/compress/snappy"
	"sync"
)

var (
	ErrMessageIsNil    = errors.New("the message is nil")
	ErrSeqOverflow     = errors.New("the message seq overflow")
	ErrRouteOverflow   = errors.New("the message route overflow")
	ErrBufferOverflow  = errors.New("the message buffer overflow")
	ErrInvalidMessage  = errors.New("invalid message")
	ErrMessageTooLarge = errors.New("the message too large")
)

type Packer interface {
	// Pack 打包消息
	Pack(message *Message) ([]byte, error)
	// Unpack 解包消息
	Unpack(data []byte) (*Message, error)
}

type defaultPacker struct {
	opts    *options
	buffers sync.Pool
	readers sync.Pool
}

func NewPacker(opts ...Option) Packer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.seqBytesLen != 0 && o.seqBytesLen != 1 && o.seqBytesLen != 2 && o.seqBytesLen != 4 {
		log.Fatalf("the seq bytes length must be 1、2、4, and give %d", o.seqBytesLen)
	}

	if o.routeBytesLen != 1 && o.routeBytesLen != 2 && o.routeBytesLen != 4 {
		log.Fatalf("the route bytes length must be 1、2、4, and give %d", o.routeBytesLen)
	}

	if o.compressBytesLen < 0 {
		log.Fatalf("the compress bytes length must be 1 and give %d", o.compressBytesLen)
	}

	if o.bufferBytesLen < 0 {
		log.Fatalf("the buffer bytes length must greater than 0, and give %d", o.bufferBytesLen)
	}

	return &defaultPacker{opts: o, buffers: sync.Pool{New: func() interface{} {
		buf := &bytes.Buffer{}
		buf.Grow(o.seqBytesLen + o.routeBytesLen + o.compressBytesLen + o.bufferBytesLen)
		return buf
	}}}
}

// Pack 打包消息
func (p *defaultPacker) Pack(message *Message) ([]byte, error) {
	if message == nil {
		return nil, ErrMessageIsNil
	}

	if p.opts.seqBytesLen > 0 {
		if message.Seq > int32(1<<(8*p.opts.seqBytesLen-1)-1) || message.Seq < int32(-1<<(8*p.opts.seqBytesLen-1)) {
			return nil, ErrSeqOverflow
		}
	}

	if message.Route > int32(1<<(8*p.opts.routeBytesLen-1)-1) || message.Route < int32(-1<<(8*p.opts.routeBytesLen-1)) {
		return nil, ErrRouteOverflow
	}

	if len(message.Buffer) > p.opts.bufferBytesLen {
		return nil, ErrBufferOverflow
	}

	var (
		err error
		buf = bytes.NewBuffer(nil)
	)

	compress := message.Compress
	buffer := message.Buffer
	// 如果消息未压缩，同时消息大小大于阈值，则 pack 时进行压缩
	if p.opts.compressEnable && !message.Compress && len(message.Buffer) > p.opts.compressThreshold {
		compress = true
		buffer, err = p.Encode(message.Buffer)
		log.Infof("seq:%+v,route:%+v,data:%+v, compressed data:%+v,err:%+v", message.Seq, message.Route, message.Buffer, buffer, err)
		if err != nil {
			return nil, err
		}
	}

	buf.Grow(p.opts.seqBytesLen + p.opts.routeBytesLen + p.opts.compressBytesLen + len(buffer))

	switch p.opts.seqBytesLen {
	case 1:
		err = binary.Write(buf, p.opts.byteOrder, int8(message.Seq))
	case 2:
		err = binary.Write(buf, p.opts.byteOrder, int16(message.Seq))
	case 4:
		err = binary.Write(buf, p.opts.byteOrder, message.Seq)
	}
	if err != nil {
		return nil, err
	}

	switch p.opts.routeBytesLen {
	case 1:
		err = binary.Write(buf, p.opts.byteOrder, int8(message.Route))
	case 2:
		err = binary.Write(buf, p.opts.byteOrder, int16(message.Route))
	case 4:
		err = binary.Write(buf, p.opts.byteOrder, message.Route)
	}
	if err != nil {
		return nil, err
	}

	// write compress
	err = binary.Write(buf, p.opts.byteOrder, compress)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, p.opts.byteOrder, buffer)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Unpack 解包消息
func (p *defaultPacker) Unpack(data []byte) (*Message, error) {
	ln := len(data) - p.opts.seqBytesLen - p.opts.routeBytesLen - p.opts.compressBytesLen

	if ln < 0 {
		return nil, ErrInvalidMessage
	}

	if ln > p.opts.bufferBytesLen {
		return nil, ErrMessageTooLarge
	}

	var (
		err     error
		reader  = bytes.NewReader(data)
		message = &Message{Buffer: make([]byte, ln)}
	)

	switch p.opts.seqBytesLen {
	case 1:
		var seq int8
		if err = binary.Read(reader, p.opts.byteOrder, &seq); err != nil {
			return nil, err
		} else {
			message.Seq = int32(seq)
		}
	case 2:
		var seq int16
		if err = binary.Read(reader, p.opts.byteOrder, &seq); err != nil {
			return nil, err
		} else {
			message.Seq = int32(seq)
		}
	case 4:
		var seq int32
		if err = binary.Read(reader, p.opts.byteOrder, &seq); err != nil {
			return nil, err
		} else {
			message.Seq = seq
		}
	}
	if err != nil {
		return nil, err
	}

	switch p.opts.routeBytesLen {
	case 1:
		var route int8
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return nil, err
		} else {
			message.Route = int32(route)
		}
	case 2:
		var route int16
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return nil, err
		} else {
			message.Route = int32(route)
		}
	case 4:
		var route int32
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return nil, err
		} else {
			message.Route = route
		}
	}
	if err != nil {
		return nil, err
	}

	// compress
	var compress bool
	if err = binary.Read(reader, p.opts.byteOrder, &compress); err != nil {
		return nil, err
	} else {
		message.Compress = compress
	}
	if err != nil {
		return nil, err
	}

	err = binary.Read(reader, p.opts.byteOrder, &message.Buffer)
	if err != nil {
		return nil, err
	}

	if p.opts.compressEnable && message.Compress {
		got, err := p.Decode(message.Buffer)
		if err != nil {
			return nil, err
		}

		message.Compress = false
		copy(message.Buffer, got)
	}

	return message, nil
}

func (p *defaultPacker) Encode(data []byte) (encodeData []byte, err error) {
	switch p.opts.compressAlgorithm {
	case "snappy":
		encodeData, err = snappy.Encode(data)
		return
	case "gzip":
		encodeData, err = gzip.Encode(data)
		return
	default:
		err = errors.New("invalid compress algorithm")
		return
	}

}

func (p *defaultPacker) Decode(data []byte) (decodeData []byte, err error) {
	switch p.opts.compressAlgorithm {
	case "snappy":
		decodeData, err = snappy.Decode(data)
		return
	case "gzip":
		decodeData, err = gzip.Decode(data)
		return
	default:
		err = errors.New("invalid compress algorithm")
		return
	}
}
