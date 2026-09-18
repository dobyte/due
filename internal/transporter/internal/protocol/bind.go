package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	bindReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B64 + def.B64
	bindResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeBindReq 编码绑定请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{cid + uid}
func EncodeBindReq(seq uint64, cid, uid int64) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(bindReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(bindReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Bind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, cid, uid)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeBindReq 解码绑定请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{cid + uid}
func DecodeBindReq(buf buffer.Buffer) (cid, uid int64, err error) {
	if buf.Len() != def.B64*2 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	cid = int64(binary.BigEndian.Uint64(data[:def.B64]))
	uid = int64(binary.BigEndian.Uint64(data[def.B64:]))

	return
}

// EncodeBindRes 编码绑定响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func EncodeBindRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(bindResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(bindResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Bind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeBindRes 解码绑定响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func DecodeBindRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
