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
)

const (
	statReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8
	statResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodeStatReq 编码统计在线人数请求
// 协议：size + header + route + seq + session kind
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
// 协议：size + header + route + seq + session kind
func DecodeStatReq(data []byte) (seq uint64, kind session.Kind, err error) {
	if len(data) != statReqBytes {
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

	if k, err = reader.ReadUint8(); err == nil {
		kind = session.Kind(k)
	}

	return
}

// EncodeStatRes 编码统计在线人数响应
// 协议：size + header + route + seq + code + [total]
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
// 协议：size + header + route + seq + code + [total]
func DecodeStatRes(data []byte) (code uint16, total uint64, err error) {
	if len(data) != statResBytes && len(data) != statResBytes-def.B64 {
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

	if code == codes.OK && len(data) == statResBytes {
		total, err = reader.ReadUint64(binary.BigEndian)
	}

	return
}
