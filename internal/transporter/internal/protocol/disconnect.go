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
	disconnectReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64 + def.B8
	disconnectResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeDisconnectReq 编码断连请求
// 注意：buf 包含全段协议
// 协议：{size + header + route + seq} + 私有段：{session kind + target + force}
func EncodeDisconnectReq(seq uint64, kind session.Kind, target int64, force bool) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(disconnectReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(disconnectReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Disconnect)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)
	writer.WriteBools(force)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeDisconnectReq 解码断连请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + target + force}
func DecodeDisconnectReq(buf buffer.Buffer) (kind session.Kind, target int64, force bool, err error) {
	if buf.Len() != def.B8+def.B64+def.B8 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	kind = session.Kind(data[0])
	target = int64(binary.BigEndian.Uint64(data[def.B8 : def.B8+def.B64]))
	force = data[def.B8+def.B64] == 1

	return
}

// EncodeDisconnectRes 编码断连响应
// 注意：buf 包含全段协议
// 协议：{size + header + route + seq} + 私有段：{code}
func EncodeDisconnectRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(disconnectResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(disconnectResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Disconnect)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeDisconnectRes 解码断连响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func DecodeDisconnectRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
