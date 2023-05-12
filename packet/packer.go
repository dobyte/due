package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/errors"
	"github.com/dobyte/due/log"
	"sync"
)

var (
	ErrMessageIsNil   = errors.New("the message is nil")
	ErrSeqOverflow    = errors.New("the message seq overflow")
	ErrRouteOverflow  = errors.New("the message route overflow")
	ErrInvalidMessage = errors.New("invalid message")
)

type Packer interface {
	// Pack 打包消息
	Pack(message *Message) ([]byte, error)
	Pack1(message *Message) ([]byte, error)
	// Unpack 解包消息
	Unpack(data []byte) (*Message, error)
}

type defaultPacker struct {
	opts *options
	bufs sync.Pool
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
		log.Fatalf("the route bytes length must be 1、2、4, and give %d", o.seqBytesLen)
	}

	return &defaultPacker{opts: o, bufs: sync.Pool{New: func() interface{} {
		buf := &bytes.Buffer{}
		buf.Grow(5000 + o.seqBytesLen + o.routeBytesLen)
		return buf
	}}}
}

// Pack 打包消息
func (p *defaultPacker) Pack1(message *Message) ([]byte, error) {
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

	var (
		buf = &bytes.Buffer{}
		err error
	)

	buf.Grow(len(message.Buffer) + p.opts.seqBytesLen + p.opts.routeBytesLen)

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

	err = binary.Write(buf, p.opts.byteOrder, message.Buffer)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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

	var (
		//buf bytes.Buffer
		err error
	)

	buf := p.bufs.Get().(*bytes.Buffer)
	defer p.bufs.Put(buf)

	//buf.Grow(len(message.Buffer) + p.opts.seqBytesLen + p.opts.routeBytesLen)

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

	err = binary.Write(buf, p.opts.byteOrder, message.Buffer)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Unpack 解包消息
func (p *defaultPacker) Unpack(data []byte) (*Message, error) {
	ln := len(data) - p.opts.seqBytesLen - p.opts.routeBytesLen

	if ln < 0 {
		return nil, ErrInvalidMessage
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

	err = binary.Read(reader, p.opts.byteOrder, &message.Buffer)
	if err != nil {
		return nil, err
	}

	return message, nil
}
