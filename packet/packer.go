package packet

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

const (
	dataBit      = 0 << 7 // 数据标识
	heartbeatBit = 1 << 7 // 心跳标识
)

// NocopyReader 无拷贝读取器接口
// 用于在读取消息时避免不必要的内存拷贝
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

// Packer 打包器接口
// 定义消息的编码与解码能力
type Packer interface {
	// ReadBuffer 以buffer的形式读取消息
	ReadBuffer(reader io.Reader) (buffer.Buffer, error)
	// PackBuffer 以buffer的形式打包消息
	PackBuffer(message *Message) (*buffer.NocopyBuffer, error)
	// ReadMessage 读取消息
	ReadMessage(reader io.Reader) ([]byte, error)
	// PackMessage 打包消息
	PackMessage(message *Message) ([]byte, error)
	// UnpackMessage 解包消息
	UnpackMessage(data []byte) (*Message, error)
	// PackHeartbeat 打包心跳
	PackHeartbeat() ([]byte, error)
	// CheckHeartbeat 检测心跳包
	CheckHeartbeat(data []byte) (bool, error)
}

// defaultPacker 默认打包器
type defaultPacker struct {
	opts      *options // 打包配置
	heartbeat []byte   // 预构建的心跳包
}

// NewPacker 创建默认打包器
// 校验配置合法性并预构建心跳包；传入参数不合法时将直接终止程序
// @param opts ...Option 打包配置选项
// @return @1 *defaultPacker 默认打包器
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

	return &defaultPacker{
		opts:      o,
		heartbeat: makeHeartbeat(o.byteOrder),
	}
}

// ReadBuffer 以buffer的形式读取消息
// 先读取4字节长度，再一次性读取完整消息体，返回封装好的缓冲区
// @param reader io.Reader 数据读取源
// @return @1 buffer.Buffer 读取到的消息缓冲区；无消息时返回nil
// @return @2 error 读取失败或数据不完整时返回的错误
func (p *defaultPacker) ReadBuffer(reader io.Reader) (buffer.Buffer, error) {
	buf1 := buffer.MallocBytes(defaultSizeBytes)
	defer buf1.Release()

	if _, err := io.ReadFull(reader, buf1.Bytes()); err != nil {
		return nil, err
	}

	size := p.opts.byteOrder.Uint32(buf1.Bytes())

	if size == 0 {
		return nil, nil
	}

	buf2 := buffer.MallocBytes(int(defaultSizeBytes + size))
	data := buf2.Bytes()

	copy(data[:defaultSizeBytes], buf1.Bytes())

	if _, err := io.ReadFull(reader, data[defaultSizeBytes:]); err != nil {
		buf2.Release()
		return nil, err
	}

	return buf2, nil
}

// PackBuffer 以buffer的形式打包消息
// 校验路由、序列号及消息长度后将消息编码为无拷贝缓冲区
// @param message *Message 待打包的消息
// @return @1 *buffer.NocopyBuffer 打包后的无拷贝缓冲区
// @return @2 error 路由/序列号溢出或消息过大时返回的错误
func (p *defaultPacker) PackBuffer(message *Message) (*buffer.NocopyBuffer, error) {
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

	writer := buffer.MallocWriter(defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes)
	writer.WriteInt32s(p.opts.byteOrder, int32(defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes+len(message.Buffer)))
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

	return buffer.NewNocopyBuffer(writer, message.Buffer), nil
}

// ReadMessage 读取消息
// 先读取4字节长度，再一次性读取完整消息体
// @param reader io.Reader 数据读取源
// @return @1 []byte 读取到的消息字节；无消息时返回nil
// @return @2 error 读取失败或数据不完整时返回的错误
func (p *defaultPacker) ReadMessage(reader io.Reader) ([]byte, error) {
	buf := make([]byte, defaultSizeBytes)

	if _, err := io.ReadFull(reader, buf); err != nil {
		return nil, err
	}

	size := p.opts.byteOrder.Uint32(buf)

	if size == 0 {
		return nil, nil
	}

	data := make([]byte, int(defaultSizeBytes+size))

	copy(data[:defaultSizeBytes], buf)

	if _, err := io.ReadFull(reader, data[defaultSizeBytes:]); err != nil {
		return nil, err
	}

	return data, nil
}

// nocopyReadMessage 无拷贝读取消息
// 通过Peek/Slice的方式直接从底层缓冲区切取消息，避免内存拷贝
// @param reader NocopyReader 无拷贝读取器
// @return @1 []byte 读取到的消息字节；无消息时返回nil
// @return @2 error 读取失败或数据不完整时返回的错误
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

// PackMessage 打包消息
// 校验路由、序列号及消息长度后按配置的字节序编码为字节数组
// @param message *Message 待打包的消息
// @return @1 []byte 打包后的消息字节
// @return @2 error 路由/序列号溢出、消息过大或编码失败时返回的错误
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

// UnpackMessage 解包消息
// 校验消息长度与数据标识后解析原始字节为消息对象
// @param data []byte 待解包的原始消息字节
// @return @1 *Message 解包后的消息对象
// @return @2 error 消息非法或解析失败时返回的错误
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
// 开启心跳时间时携带当前时间戳，否则返回预构建的心跳包
// @return @1 []byte 心跳包字节
// @return @2 error 编码失败时返回的错误
func (p *defaultPacker) PackHeartbeat() ([]byte, error) {
	if p.opts.heartbeatTime {
		var (
			buf  = &bytes.Buffer{}
			size = defaultHeaderBytes + defaultHeartbeatTimeBytes
		)

		buf.Grow(defaultSizeBytes + size)

		if err := binary.Write(buf, p.opts.byteOrder, uint32(size)); err != nil {
			return nil, err
		}

		if err := binary.Write(buf, p.opts.byteOrder, uint8(heartbeatBit)); err != nil {
			return nil, err
		}

		if err := binary.Write(buf, p.opts.byteOrder, time.Now().UnixNano()); err != nil {
			return nil, err
		}

		return buf.Bytes(), nil
	} else {
		return p.heartbeat, nil
	}
}

// CheckHeartbeat 检测心跳包
// 校验消息长度与心跳标识，判断给定数据是否为心跳包
// @param data []byte 待检测的消息字节
// @return @1 bool 是否为心跳包
// @return @2 error 消息非法或解析失败时返回的错误
func (p *defaultPacker) CheckHeartbeat(data []byte) (bool, error) {
	if len(data) < defaultSizeBytes+defaultHeaderBytes {
		return false, errors.ErrInvalidMessage
	}

	var (
		size   uint32
		header uint8
		reader = bytes.NewReader(data)
	)

	if err := binary.Read(reader, p.opts.byteOrder, &size); err != nil {
		return false, err
	}

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return false, errors.ErrInvalidMessage
	}

	if err := binary.Read(reader, p.opts.byteOrder, &header); err != nil {
		return false, err
	}

	return header&heartbeatBit == heartbeatBit, nil
}

// makeHeartbeat 构建心跳包
// 按指定字节序生成不含时间戳的基础心跳包
// @param byteOrder binary.ByteOrder 字节序
// @return @1 []byte 心跳包字节
func makeHeartbeat(byteOrder binary.ByteOrder) []byte {
	buf := bytes.NewBuffer(nil)
	buf.Grow(defaultSizeBytes + defaultHeaderBytes)

	_ = binary.Write(buf, byteOrder, uint32(defaultHeaderBytes))
	_ = binary.Write(buf, byteOrder, uint8(heartbeatBit))

	return buf.Bytes()
}
