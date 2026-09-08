package protocol

import (
	"encoding/binary"
	"io"

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
// 协议：size + header + route + seq + session kind + target
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
// 协议：size + header + route + seq + session kind + target
func DecodeGetIPReq(data []byte) (seq uint64, kind session.Kind, target int64, err error) {
	if len(data) != getIPReqBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(def.SizeBytes+def.HeaderBytes+def.RouteBytes, io.SeekStart); err != nil {
		return
	}

	if seq, err = reader.ReadUint64(binary.BigEndian); err != nil {
		return
	}

	var k uint8
	if k, err = reader.ReadUint8(); err != nil {
		return
	} else {
		kind = session.Kind(k)
	}

	if target, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	return
}

// EncodeGetIPRes 编码获取IP响应
// 协议：size + header + route + seq + code + [ip]
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
// 协议：size + header + route + seq + code + [ip]
func DecodeGetIPRes(data []byte) (code uint16, ip string, err error) {
	if len(data) != getIPResBytes && len(data) != getIPResBytes-def.B32 {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes, io.SeekStart); err != nil {
		return
	}

	if code, err = reader.ReadUint16(binary.BigEndian); err != nil {
		return
	}

	if code == codes.OK && len(data) == getIPResBytes {
		var v uint32
		if v, err = reader.ReadUint32(binary.BigEndian); err != nil {
			return
		} else {
			ip = xnet.Long2IP(v)
		}
	}

	return
}
