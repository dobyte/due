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
	multicastReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B16 + def.B8
	multicastResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodeMulticastReq 编码组播请求（最多组播65535个对象）
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + count + targets + disconnect + <message packet>}
func EncodeMulticastReq(seq uint64, kind session.Kind, targets []int64, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	size := multicastReqBytes + len(targets)*def.B64

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes+message.Len()))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Multicast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteUint16s(binary.BigEndian, uint16(len(targets)))
	writer.WriteInt64s(binary.BigEndian, targets...)
	writer.WriteBools(disconnect)

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodeMulticastReq 解码组播请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + count + targets + disconnect + <message packet>}
func DecodeMulticastReq(req *buffer.Bytes) (session.Kind, []int64, bool, *buffer.Bytes, error) {
	if req.Len() < def.B8+def.B16+def.B8 {
		return 0, nil, false, nil, errors.ErrInvalidMessage
	}

	data := req.Bytes()
	kind := session.Kind(data[0])
	count := int(binary.BigEndian.Uint16(data[def.B8 : def.B8+def.B16]))

	offset := def.B8 + def.B16 + count*def.B64
	if req.Len() < offset+def.B8 {
		return 0, nil, false, nil, errors.ErrInvalidMessage
	}

	targets := make([]int64, count)
	for i := range count {
		start := def.B8 + def.B16 + i*def.B64
		targets[i] = int64(binary.BigEndian.Uint64(data[start : start+def.B64]))
	}

	disconnect := data[offset] == 1

	req.MoveTo(offset + def.B8)

	return kind, targets, disconnect, req, nil
}

// EncodeMulticastRes 编码组播响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [total]}
func EncodeMulticastRes(seq uint64, code uint16, total ...uint64) *buffer.NocopyBuffer {
	size := multicastResBytes - def.SizeBytes
	if code != codes.OK || len(total) == 0 || total[0] == 0 {
		size -= def.B64
	}

	writer := buffer.MallocWriter(multicastResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Multicast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(total) > 0 && total[0] != 0 {
		writer.WriteUint64s(binary.BigEndian, total[0])
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeMulticastRes 解码组播响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [total]}
func DecodeMulticastRes(buf buffer.Buffer) (code uint16, total uint64, err error) {
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
