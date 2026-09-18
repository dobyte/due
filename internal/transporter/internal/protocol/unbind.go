package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	unbindReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B64
	unbindResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeUnbindReq 编码解绑请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{uid}
func EncodeUnbindReq(seq uint64, uid int64) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(unbindReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unbindReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Unbind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, uid)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnbindReq 解码解绑请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{uid}
func DecodeUnbindReq(buf buffer.Buffer) (uid int64, err error) {
	if buf.Len() != def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	uid = int64(binary.BigEndian.Uint64(buf.Bytes()))

	return
}

// EncodeUnbindRes 编码解绑响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func EncodeUnbindRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(unbindResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unbindResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Unbind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnbindRes 解码解绑响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func DecodeUnbindRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
