package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	handshakeReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64
	handshakeResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeHandshakeReq 编码握手请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{ins kind + ins id + conn epoch}
func EncodeHandshakeReq(seq uint64, kind cluster.Kind, inst string, epoch uint64) *buffer.NocopyBuffer {
	size := handshakeReqBytes + len(inst)

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Handshake)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteString(inst)
	writer.WriteUint64s(binary.BigEndian, epoch)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeHandshakeReq 解码握手请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{ins kind + ins id + conn epoch}
func DecodeHandshakeReq(buf *buffer.Bytes) (kind cluster.Kind, inst string, epoch uint64, err error) {
	var (
		k      uint8
		reader = buffer.NewReader(buf.Bytes())
	)

	if k, err = reader.ReadUint8(); err != nil {
		return
	} else {
		kind = cluster.Kind(k)
	}

	if inst, err = reader.ReadString(buf.Len() - def.B8 - def.B64); err != nil {
		return
	}

	epoch, err = reader.ReadUint64(binary.BigEndian)

	return
}

// EncodeHandshakeRes 编码握手响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code}
func EncodeHandshakeRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(handshakeResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(handshakeResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Handshake)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeHandshakeRes 解码握手响应
// 协议：size + header + route + seq + code
func DecodeHandshakeRes(data []byte) (code uint16, err error) {
	if len(data) != handshakeResBytes {
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
