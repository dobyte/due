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
	pushReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64 + def.B8
	pushResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodePushReq 编码推送请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + target + disconnect + <message packet>}
func EncodePushReq(seq uint64, kind session.Kind, target int64, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(pushReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(pushReqBytes-def.SizeBytes+message.Len()))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Push)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)
	writer.WriteBools(disconnect)

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodePushReq 解码推送消息
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + target + disconnect + <message packet>}
func DecodePushReq(req *buffer.Bytes) (session.Kind, int64, bool, *buffer.Bytes, error) {
	if req.Len() < def.B8+def.B64+def.B8 {
		return 0, 0, false, nil, errors.ErrInvalidMessage
	}

	data := req.Bytes()
	kind := session.Kind(data[0])
	target := int64(binary.BigEndian.Uint64(data[def.B8 : def.B8+def.B64]))
	disconnect := data[def.B8+def.B64] == 1

	req.MoveTo(def.B8 + def.B64 + def.B8)

	return kind, target, disconnect, req, nil
}

// EncodePushRes 编码推送响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func EncodePushRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(pushResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(pushResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Push)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodePushRes 解码推送响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func DecodePushRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
