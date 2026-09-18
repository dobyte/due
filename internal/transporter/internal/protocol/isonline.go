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
	isOnlineReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64
	isOnlineResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B8
)

// EncodeIsOnlineReq 编码检测用户是否在线请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + target}
func EncodeIsOnlineReq(seq uint64, kind session.Kind, target int64) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(isOnlineReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(isOnlineReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.IsOnline)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeIsOnlineReq 解码检测用户是否在线请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + target}
func DecodeIsOnlineReq(buf buffer.Buffer) (kind session.Kind, target int64, err error) {
	if buf.Len() != def.B8+def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	kind = session.Kind(data[0])
	target = int64(binary.BigEndian.Uint64(data[def.B8:]))

	return
}

// EncodeIsOnlineRes 编码检测用户是否在线响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code + online state}
func EncodeIsOnlineRes(seq uint64, code uint16, isOnline bool) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(isOnlineResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(isOnlineResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.IsOnline)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)
	writer.WriteBools(isOnline)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeIsOnlineRes 解码检测用户是否在线响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code + online state}
func DecodeIsOnlineRes(buf buffer.Buffer) (code uint16, isOnline bool, err error) {
	if buf.Len() != def.CodeBytes+def.B8 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	code = binary.BigEndian.Uint16(data[:def.CodeBytes])
	isOnline = data[def.CodeBytes] == 1

	return
}
