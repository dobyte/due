package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"io"
	"sync"
	"time"
)

const (
	dataBit      = 0 << 7 // 数据标识
	heartbeatBit = 1 << 7 // 心跳标识
)

type NocopyReader interface {
	// Next returns a slice containing the next n bytes from the buffer,
	// advancing the buffer as if the bytes had been returned by Read.
	Next(n int) (p []byte, err error)

	// Peek returns the next n bytes without advancing the reader.
	Peek(n int) (buf []byte, err error)

	// Release the memory space occupied by all read slices.
	Release() (err error)

	Slice(n int) (r NocopyReader, err error)
}

type Packer interface {
	// ReadMessage 读取消息
	ReadMessage(reader interface{}) ([]byte, error)
	// PackMessage 打包消息
	PackMessage(message *Message) ([]byte, error)
	PackMessage2(message *Message) (*buffer.Buffer, error)
	// UnpackMessage 解包消息
	UnpackMessage(data []byte) (*Message, error)
	// PackHeartbeat 打包心跳
	PackHeartbeat() ([]byte, error)
	// CheckHeartbeat 检测心跳包
	CheckHeartbeat(data []byte) (bool, error)
	// ExtractRoute 提取路由
	ExtractRoute(data []byte) (int32, error)
}

type defaultPacker struct {
	opts             *options
	once             sync.Once
	heartbeat        []byte
	readerSizePool   sync.Pool
	readerBufferPool sync.Pool
}

func NewPacker(opts ...Option) *defaultPacker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.routeBytes != 1 && o.routeBytes != 2 && o.routeBytes != 4 {
		log.Fatalf("the number of route bytes must be 1、2、4, and give %d", o.routeBytes)
	}

	if o.seqBytes != 0 && o.seqBytes != 1 && o.seqBytes != 2 && o.seqBytes != 4 {
		log.Fatalf("the number of seq bytes must be 0、1、2、4, and give %d", o.seqBytes)
	}

	if o.bufferBytes < 0 {
		log.Fatalf("the number of buffer bytes must be greater than or equal to 0, and give %d", o.bufferBytes)
	}

	p := &defaultPacker{opts: o}

	if !o.heartbeatTime {
		buf := &bytes.Buffer{}

		buf.Grow(defaultSizeBytes + defaultHeaderBytes)

		_ = binary.Write(buf, o.byteOrder, uint32(defaultHeaderBytes))

		_ = binary.Write(buf, o.byteOrder, uint8(heartbeatBit))

		p.heartbeat = buf.Bytes()
	}

	p.readerSizePool = sync.Pool{New: func() any { return make([]byte, defaultSizeBytes) }}

	p.readerBufferPool = sync.Pool{New: func() any {
		return make([]byte, defaultSizeBytes+defaultHeaderBytes+o.routeBytes+o.seqBytes+o.bufferBytes)
	}}

	return p
}

// ReadMessage 读取消息
func (p *defaultPacker) ReadMessage(reader interface{}) ([]byte, error) {
	switch r := reader.(type) {
	case NocopyReader:
		return p.nocopyReadMessage(r)
	case io.Reader:
		return p.copyReadMessage(r)
	default:
		return nil, errors.ErrInvalidReader
	}
}

// 无拷贝读取消息
func (p *defaultPacker) nocopyReadMessage(reader NocopyReader) ([]byte, error) {
	buf, err := reader.Peek(defaultSizeBytes)
	if err != nil {
		return nil, err
	}

	var size uint32

	if p.opts.byteOrder == binary.BigEndian {
		size = binary.BigEndian.Uint32(buf)
	} else {
		size = binary.LittleEndian.Uint32(buf)
	}

	if size == 0 {
		return nil, nil
	}

	n := int(defaultSizeBytes + size)

	r, err := reader.Slice(n)
	if err != nil {
		return nil, err
	}

	buf, err = r.Next(n)
	if err != nil {
		return nil, err
	}

	if err = reader.Release(); err != nil {
		return nil, err
	}

	return buf, nil
}

// 拷贝读取消息
func (p *defaultPacker) copyReadMessage(reader io.Reader) ([]byte, error) {
	buf := make([]byte, defaultSizeBytes)

	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, err
	}

	var size uint32

	if p.opts.byteOrder == binary.BigEndian {
		size = binary.BigEndian.Uint32(buf)
	} else {
		size = binary.LittleEndian.Uint32(buf)
	}

	if size == 0 {
		return nil, nil
	}

	data := make([]byte, defaultSizeBytes+size)
	copy(data[:defaultSizeBytes], buf)

	_, err = io.ReadFull(reader, data[defaultSizeBytes:])
	if err != nil {
		return nil, err
	}

	return data, nil
}

// PackMessage 打包消息
func (p *defaultPacker) PackMessage(message *Message) ([]byte, error) {
	if message.Route > int32(1<<(8*p.opts.routeBytes-1)-1) || message.Route < int32(-1<<(8*p.opts.routeBytes-1)) {
		return nil, errors.ErrRouteOverflow
	}

	if p.opts.seqBytes > 0 {
		if message.Seq > int32(1<<(8*p.opts.seqBytes-1)-1) || message.Seq < int32(-1<<(8*p.opts.seqBytes-1)) {
			return nil, errors.ErrSeqOverflow
		}
	}

	if len(message.Buffer) > p.opts.bufferBytes {
		return nil, errors.ErrMessageTooLarge
	}

	var (
		size = defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes + len(message.Buffer)
		buf  = &bytes.Buffer{}
	)

	buf.Grow(size + defaultSizeBytes)

	err := binary.Write(buf, p.opts.byteOrder, int32(size))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, p.opts.byteOrder, int8(dataBit))
	if err != nil {
		return nil, err
	}

	switch p.opts.routeBytes {
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

	switch p.opts.seqBytes {
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

	err = binary.Write(buf, p.opts.byteOrder, message.Buffer)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (p *defaultPacker) PackMessage2(message *Message) (*buffer.Buffer, error) {
	if message.Route > int32(1<<(8*p.opts.routeBytes-1)-1) || message.Route < int32(-1<<(8*p.opts.routeBytes-1)) {
		return nil, errors.ErrRouteOverflow
	}

	if p.opts.seqBytes > 0 {
		if message.Seq > int32(1<<(8*p.opts.seqBytes-1)-1) || message.Seq < int32(-1<<(8*p.opts.seqBytes-1)) {
			return nil, errors.ErrSeqOverflow
		}
	}

	if len(message.Buffer) > p.opts.bufferBytes {
		return nil, errors.ErrMessageTooLarge
	}

	var (
		size = defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes + len(message.Buffer)
		buf  = buffer.NewBuffer()
	)

	writer := buf.Malloc(defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes)
	writer.WriteInt32s(p.opts.byteOrder, int32(size))
	writer.WriteInt8s(int8(dataBit))

	switch p.opts.routeBytes {
	case 1:
		writer.WriteInt8s(int8(message.Route))
	case 2:
		writer.WriteInt16s(p.opts.byteOrder, int16(message.Route))
	case 4:
		writer.WriteInt32s(p.opts.byteOrder, message.Route)
	}

	switch p.opts.seqBytes {
	case 1:
		writer.WriteInt8s(int8(message.Seq))
	case 2:
		writer.WriteInt16s(p.opts.byteOrder, int16(message.Seq))
	case 4:
		writer.WriteInt32s(p.opts.byteOrder, message.Seq)
	}

	buf.Mount(message.Buffer)

	return buf, nil
}

// ExtractRoute 提取路由
func (p *defaultPacker) ExtractRoute(data []byte) (int32, error) {
	var (
		ln     = len(data) - defaultSizeBytes - defaultHeaderBytes - p.opts.routeBytes - p.opts.seqBytes
		reader = bytes.NewReader(data)
		size   uint32
		header uint8
	)

	if ln < 0 {
		return 0, errors.ErrInvalidMessage
	}

	err := binary.Read(reader, p.opts.byteOrder, &size)
	if err != nil {
		return 0, err
	}

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return 0, errors.ErrInvalidMessage
	}

	err = binary.Read(reader, p.opts.byteOrder, &header)
	if err != nil {
		return 0, err
	}

	if header&dataBit != dataBit {
		return 0, errors.ErrInvalidMessage
	}

	switch p.opts.routeBytes {
	case 1:
		var route int8
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return 0, err
		} else {
			return int32(route), nil
		}
	case 2:
		var route int16
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return 0, err
		} else {
			return int32(route), nil
		}
	case 4:
		var route int32
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return 0, err
		} else {
			return route, nil
		}
	}

	return 0, errors.ErrInvalidMessage
}

// UnpackMessage 解包消息
func (p *defaultPacker) UnpackMessage(data []byte) (*Message, error) {
	var (
		ln     = defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes
		reader = bytes.NewReader(data)
		size   uint32
		header uint8
	)

	if len(data)-ln < 0 {
		return nil, errors.ErrInvalidMessage
	}

	err := binary.Read(reader, p.opts.byteOrder, &size)
	if err != nil {
		return nil, err
	}

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return nil, errors.ErrInvalidMessage
	}

	err = binary.Read(reader, p.opts.byteOrder, &header)
	if err != nil {
		return nil, err
	}

	if header&dataBit != dataBit {
		return nil, errors.ErrInvalidMessage
	}

	message := &Message{}

	switch p.opts.routeBytes {
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

	switch p.opts.seqBytes {
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

	message.Buffer = data[ln:]

	return message, nil
}

// PackHeartbeat 打包心跳
func (p *defaultPacker) PackHeartbeat() ([]byte, error) {
	if !p.opts.heartbeatTime {
		return p.heartbeat, nil
	}

	var (
		buf  = &bytes.Buffer{}
		size = defaultHeaderBytes + defaultHeartbeatTimeBytes
	)

	buf.Grow(defaultSizeBytes + size)

	err := binary.Write(buf, p.opts.byteOrder, uint32(size))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, p.opts.byteOrder, uint8(heartbeatBit))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, p.opts.byteOrder, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// CheckHeartbeat 检测心跳包
func (p *defaultPacker) CheckHeartbeat(data []byte) (bool, error) {
	if len(data) < defaultSizeBytes+defaultHeaderBytes {
		return false, errors.ErrInvalidMessage
	}

	var (
		size   uint32
		header uint8
		reader = bytes.NewReader(data)
	)

	err := binary.Read(reader, p.opts.byteOrder, &size)
	if err != nil {
		return false, err
	}

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return false, errors.ErrInvalidMessage
	}

	err = binary.Read(reader, p.opts.byteOrder, &header)
	if err != nil {
		return false, err
	}

	return header&heartbeatBit == heartbeatBit, nil
}
