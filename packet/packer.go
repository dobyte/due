package packet

import (
	"io"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

const (
	dataBit          = 0 << 7 // 数据标识
	heartbeatBit     = 1 << 7 // 心跳标识
	heartbeatTimeBit = 1 << 6 // 心跳时间戳标识
)

// Packer 打包器接口
// 定义消息的编码与解码能力
type Packer interface {
	// Read 以buffer的形式读取消息
	// @param reader io.Reader 数据读取源
	// @return @1 bool 是否为心跳消息
	// @return @2 int64 服务器侧时间戳（纳秒）
	// @return @3 buffer.Buffer 消息缓冲区
	// @return @4 error 读取失败时返回的错误
	Read(reader io.Reader) (bool, int64, buffer.Buffer, error)
	// PackMessage 以buffer的形式打包消息
	// @param message *Message 消息
	// @return @1 buffer.Buffer 打包后的消息缓冲区
	// @return @2 error 打包失败时返回的错误
	PackMessage(message *Message) (buffer.Buffer, error)
	// ExtractRouteSeq 从消息缓冲区中提取路由与序列号
	// @param buf buffer.Buffer 消息缓冲区
	// @return @1 int32 路由
	// @return @2 int32 序列号
	// @return @3 error 解包失败时返回的错误
	ExtractRouteSeq(buf buffer.Buffer) (int32, int32, error)
	// UnpackMessage 解包消息
	// @param buf buffer.Buffer 消息缓冲区
	// @return @1 *Message 消息对象
	// @return @2 error 解包失败时返回的错误
	UnpackMessage(buf buffer.Buffer) (int32, int32, buffer.Buffer, error)
	// PackHeartbeat 打包心跳
	// @param server ...bool 是否为服务端心跳
	// @return @1 buffer.Buffer 心跳包缓冲区
	PackHeartbeat(server ...bool) buffer.Buffer
}

// defaultPacker 默认打包器
type defaultPacker struct {
	opts      *options      // 打包配置
	heartbeat buffer.Buffer // 预构建的心跳包
}

var _ Packer = (*defaultPacker)(nil)

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

	p := &defaultPacker{}
	p.opts = o
	p.init()

	return p
}

// Read 以buffer的形式读取消息
// @param reader io.Reader 数据读取源
// @return @1 bool 是否为心跳消息
// @return @2 int64 服务器侧时间戳（纳秒）
// @return @3 buffer.Buffer 消息缓冲区
// @return @4 error 读取失败时返回的错误
func (p *defaultPacker) Read(reader io.Reader) (bool, int64, buffer.Buffer, error) {
	buf1 := buffer.MallocBytes(defaultSizeBytes + defaultHeaderBytes)

	if buf1 == nil {
		return false, 0, nil, errors.ErrMessageTooLarge
	}

	defer buf1.Release()

	data1 := buf1.Bytes()

	if _, err := io.ReadFull(reader, data1); err != nil {
		return false, 0, nil, err
	}

	var (
		header              = data1[defaultSizeBytes:][0]
		isHeartbeat         = header&heartbeatBit == heartbeatBit
		isWithHeartbeatTime = header&heartbeatTimeBit == heartbeatTimeBit
	)

	if isHeartbeat {
		if isWithHeartbeatTime {
			size := p.opts.byteOrder.Uint32(data1[:defaultSizeBytes])

			if size <= 0 {
				return false, 0, nil, errors.ErrInvalidMessage
			}

			size -= defaultHeaderBytes

			if size != defaultHeartbeatTimeBytes {
				return false, 0, nil, errors.ErrInvalidMessage
			}

			buf2 := buffer.MallocBytes(int(defaultHeartbeatTimeBytes))

			if buf2 == nil {
				return false, 0, nil, errors.ErrMessageTooLarge
			}

			defer buf2.Release()

			data2 := buf2.Bytes()

			if _, err := io.ReadFull(reader, data2); err != nil {
				return false, 0, nil, err
			}

			heartbeatTime := int64(p.opts.byteOrder.Uint64(data2))

			return true, heartbeatTime, nil, nil
		} else {
			return true, 0, nil, nil
		}
	} else {
		size := int64(p.opts.byteOrder.Uint32(data1[:defaultSizeBytes]))

		if size <= 0 {
			return false, 0, nil, errors.ErrInvalidMessage
		}

		if size > int64(defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes+p.opts.bufferBytes) {
			return false, 0, nil, errors.ErrMessageTooLarge
		}

		size -= defaultHeaderBytes

		buf2 := buffer.MallocBytes(int(defaultSizeBytes + defaultHeaderBytes + size))

		if buf2 == nil {
			return false, 0, nil, errors.ErrMessageTooLarge
		}

		data2 := buf2.Bytes()

		copy(data2[:defaultSizeBytes+defaultHeaderBytes], data1)

		if _, err := io.ReadFull(reader, data2[defaultSizeBytes+defaultHeaderBytes:]); err != nil {
			buf2.Release()
			return false, 0, nil, err
		}

		return false, 0, buf2, nil
	}
}

// PackMessage 以buffer的形式打包消息
// @param message *Message 消息
// @return @1 buffer.Buffer 打包后的消息缓冲区
// @return @2 error 打包失败时返回的错误
func (p *defaultPacker) PackMessage(message *Message) (buffer.Buffer, error) {
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
	default:
		writer.Release()
		return nil, errors.ErrInvalidMessage
	}

	switch p.opts.seqBytes {
	case 0:
		// ignore seq
	case 1:
		writer.WriteInt8s(int8(message.Seq))
	case 2:
		writer.WriteInt16s(p.opts.byteOrder, int16(message.Seq))
	case 4:
		writer.WriteInt32s(p.opts.byteOrder, message.Seq)
	default:
		writer.Release()
		return nil, errors.ErrInvalidMessage
	}

	return buffer.NewNocopyBuffer(writer, message.Buffer), nil
}

// ExtractRouteSeq 从消息缓冲区中提取路由与序列号
// @param buf buffer.Buffer 消息缓冲区
// @return @1 int32 路由
// @return @2 int32 序列号
// @return @3 error 解包失败时返回的错误
func (p *defaultPacker) ExtractRouteSeq(buf buffer.Buffer) (int32, int32, error) {
	var (
		ln   = defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes
		data = buf.Bytes()
	)

	if len(data) < ln {
		return 0, 0, errors.ErrInvalidMessage
	}

	size := p.opts.byteOrder.Uint32(data[:defaultSizeBytes])

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return 0, 0, errors.ErrInvalidMessage
	}

	header := data[defaultSizeBytes]

	if header&heartbeatBit == heartbeatBit {
		return 0, 0, errors.ErrInvalidMessage
	}

	var route int32

	switch p.opts.routeBytes {
	case 1:
		route = int32(data[defaultSizeBytes+defaultHeaderBytes])
	case 2:
		route = int32(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	case 4:
		route = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	default:
		return 0, 0, errors.ErrInvalidMessage
	}

	var seq int32

	switch p.opts.seqBytes {
	case 0:
		// ignore seq
	case 1:
		seq = int32(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes])
	case 2:
		seq = int32(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes]))
	case 4:
		seq = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes]))
	default:
		return 0, 0, errors.ErrInvalidMessage
	}

	return route, seq, nil
}

// UnpackMessage 解包消息
// @param data []byte 待解包的原始消息缓冲区
// @return @1 int32 路由
// @return @2 int32 序列号
// @return @3 *buffer.Bytes 消息内容
// @return @4 error 解包失败时返回的错误
func (p *defaultPacker) UnpackMessage(buf buffer.Buffer) (int32, int32, buffer.Buffer, error) {
	var (
		ln   = defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes
		data = buf.Bytes()
	)

	if len(data) < ln {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	size := p.opts.byteOrder.Uint32(data[:defaultSizeBytes])

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	header := data[defaultSizeBytes]

	if header&heartbeatBit == heartbeatBit {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	var route int32

	switch p.opts.routeBytes {
	case 1:
		route = int32(data[defaultSizeBytes+defaultHeaderBytes])
	case 2:
		route = int32(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	case 4:
		route = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	default:
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	var seq int32

	switch p.opts.seqBytes {
	case 0:
		// ignore seq
	case 1:
		seq = int32(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes])
	case 2:
		seq = int32(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes]))
	case 4:
		seq = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes]))
	default:
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	buf.Slide(ln)

	return route, seq, buf, nil
}

// PackHeartbeat 打包心跳
// 开启心跳时间时携带当前时间戳，否则返回预构建的心跳包
// 注意：无心跳时间时返回的为共享预构建缓冲区，调用方不可修改或释放
// @return @1 buffer.Buffer 心跳包字节
func (p *defaultPacker) PackHeartbeat(server ...bool) buffer.Buffer {
	if p.opts.heartbeatTime && len(server) > 0 && server[0] {
		writer := buffer.MallocWriter(defaultSizeBytes + defaultHeaderBytes + defaultHeartbeatTimeBytes)
		writer.WriteUint32s(p.opts.byteOrder, uint32(defaultHeaderBytes+defaultHeartbeatTimeBytes))
		writer.WriteUint8s(uint8(heartbeatBit | heartbeatTimeBit))
		writer.WriteUint64s(p.opts.byteOrder, uint64(time.Now().UnixNano()))

		return writer
	} else {
		return p.heartbeat
	}
}

// init 初始化打包器
// 按指定字节序生成不含时间戳的基础心跳包
func (p *defaultPacker) init() {
	writer := buffer.NewWriter(make([]byte, defaultSizeBytes+defaultHeaderBytes), true)
	writer.WriteUint32s(p.opts.byteOrder, uint32(defaultHeaderBytes))
	writer.WriteUint8s(uint8(heartbeatBit))

	p.heartbeat = writer
}
