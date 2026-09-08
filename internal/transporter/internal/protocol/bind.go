package protocol

import (
	"encoding/binary"
	"io"

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
// 协议：size + header + route + seq + cid + uid
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
// 协议：size + header + route + seq + cid + uid
func DecodeBindReq(data []byte) (seq uint64, cid, uid int64, err error) {
	if len(data) != bindReqBytes {
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

	if cid, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	if uid, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	return
}

// EncodeBindRes 编码绑定响应
// 协议：size + header + route + seq + code
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
// 协议：size + header + route + seq + code
func DecodeBindRes(data []byte) (code uint16, err error) {
	if len(data) != bindResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(-def.CodeBytes, io.SeekEnd); err != nil {
		return
	}

	if code, err = reader.ReadUint16(binary.BigEndian); err != nil {
		return
	}

	return
}
