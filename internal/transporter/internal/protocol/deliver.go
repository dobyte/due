package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	deliverReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B64 + def.B64
	deliverResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeDeliverReq 编码投递消息请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{cid + uid + <message packet>}
func EncodeDeliverReq(seq uint64, cid int64, uid int64, buf buffer.Buffer) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(deliverReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(deliverReqBytes-def.SizeBytes+buf.Len()))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Deliver)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, cid, uid)

	return buffer.NewNocopyBuffer(writer, buf)
}

// DecodeDeliverReq 解码投递消息请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{cid + uid + <message packet>}
func DecodeDeliverReq(req *buffer.Bytes) (int64, int64, *buffer.Bytes, error) {
	if req.Len() < def.B64*2 {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	data := req.Bytes()
	cid := int64(binary.BigEndian.Uint64(data[:def.B64]))
	uid := int64(binary.BigEndian.Uint64(data[def.B64 : def.B64*2]))

	req.MoveTo(def.B64 * 2)

	return cid, uid, req, nil
}

// EncodeDeliverRes 编码投递消息响应
// 协议：size + header + route + seq + code
func EncodeDeliverRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(deliverResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(deliverResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Deliver)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeDeliverRes 解码投递消息响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func DecodeDeliverRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
