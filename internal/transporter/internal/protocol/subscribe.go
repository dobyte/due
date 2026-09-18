package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

const (
	subscribeReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B16
	subscribeResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeSubscribeReq 编码订阅频道请求（单次最多订阅65535个对象）
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + count + targets + channel}
func EncodeSubscribeReq(seq uint64, kind session.Kind, targets []int64, channel string) *buffer.NocopyBuffer {
	size := subscribeReqBytes + len(targets)*def.B64 + len([]byte(channel))

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Subscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteUint16s(binary.BigEndian, uint16(len(targets)))
	writer.WriteInt64s(binary.BigEndian, targets...)
	writer.WriteString(channel)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeSubscribeReq 解码订阅频道请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + count + targets + channel}
func DecodeSubscribeReq(buf buffer.Buffer) (kind session.Kind, targets []int64, channel string, err error) {
	data := buf.Bytes()

	if len(data) < def.B8+def.B16 {
		err = errors.ErrInvalidMessage
		return
	}

	kind = session.Kind(data[0])
	count := int(binary.BigEndian.Uint16(data[def.B8 : def.B8+def.B16]))

	offset := def.B8 + def.B16 + count*def.B64
	if len(data) < offset {
		err = errors.ErrInvalidMessage
		return
	}

	targets = make([]int64, count)
	for i := 0; i < count; i++ {
		start := def.B8 + def.B16 + i*def.B64
		targets[i] = int64(binary.BigEndian.Uint64(data[start : start+def.B64]))
	}

	channel = string(data[offset:])

	return
}

// EncodeSubscribeRes 编码订阅频道响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func EncodeSubscribeRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(subscribeResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(subscribeResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Subscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeSubscribeRes 解码订阅频道响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func DecodeSubscribeRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
