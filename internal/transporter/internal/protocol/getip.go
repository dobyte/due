package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/utils/xnet"
)

const (
	getIPReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64
	getIPResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B32
)

// EncodeGetIPReq 编码获取IP请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + target}
func EncodeGetIPReq(seq uint64, kind session.Kind, target int64) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(getIPReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(getIPReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.GetIP)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeGetIPReq 解码获取IP请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{session kind + target}
func DecodeGetIPReq(buf buffer.Buffer) (kind session.Kind, target int64, err error) {
	if buf.Len() != def.B8+def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	kind = session.Kind(data[0])
	target = int64(binary.BigEndian.Uint64(data[def.B8:]))

	return
}

// EncodeGetIPRes 编码获取IP响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [ip]}
func EncodeGetIPRes(seq uint64, code uint16, ip ...string) *buffer.NocopyBuffer {
	size := getIPResBytes - def.SizeBytes
	if code != codes.OK || len(ip) == 0 || ip[0] == "" {
		size -= def.B32
	}

	writer := buffer.MallocWriter(getIPResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.GetIP)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(ip) > 0 && ip[0] != "" {
		writer.WriteUint32s(binary.BigEndian, xnet.IP2Long(ip[0]))
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeGetIPRes 解码获取IP响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [ip]}
func DecodeGetIPRes(buf buffer.Buffer) (code uint16, ip string, err error) {
	if buf.Len() != def.CodeBytes && buf.Len() != def.CodeBytes+def.B32 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	code = binary.BigEndian.Uint16(data[:def.CodeBytes])

	if code == codes.OK && buf.Len() == def.CodeBytes+def.B32 {
		ip = xnet.Long2IP(binary.BigEndian.Uint32(data[def.CodeBytes:]))
	}

	return
}
