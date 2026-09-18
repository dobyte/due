package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

const (
	statReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8
	statResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodeStatReq 编码统计在线人数请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind}
func EncodeStatReq(seq uint64, kind session.Kind) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(statReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(statReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Stat)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))

	return buffer.NewNocopyBuffer(writer)
}

// DecodeStatReq 解码统计在线人数请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind}
func DecodeStatReq(buf buffer.Buffer) (kind session.Kind, err error) {
	if buf.Len() != def.B8 {
		err = errors.ErrInvalidMessage
		return
	}

	kind = session.Kind(buf.Bytes()[0])

	return
}

// EncodeStatRes 编码统计在线人数响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [total]}
func EncodeStatRes(seq uint64, code uint16, total ...uint64) *buffer.NocopyBuffer {
	size := statResBytes - def.SizeBytes
	if code != codes.OK || len(total) == 0 || total[0] == 0 {
		size -= def.B64
	}

	writer := buffer.MallocWriter(statResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Stat)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(total) > 0 && total[0] != 0 {
		writer.WriteUint64s(binary.BigEndian, total[0])
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeStatRes 解码统计在线人数响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [total]}
func DecodeStatRes(buf buffer.Buffer) (code uint16, total uint64, err error) {
	if buf.Len() != def.CodeBytes && buf.Len() != def.CodeBytes+def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	code = binary.BigEndian.Uint16(data[:def.CodeBytes])

	if code == codes.OK && buf.Len() == def.CodeBytes+def.B64 {
		total = binary.BigEndian.Uint64(data[def.CodeBytes:])
	}

	return
}
