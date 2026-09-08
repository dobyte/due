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
	multicastReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B16
	multicastResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodeMulticastReq 编码组播请求（最多组播65535个对象）
// 协议：size + header + route + seq + session kind + count + targets + <message packet>
func EncodeMulticastReq(seq uint64, kind session.Kind, targets []int64, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	size := multicastReqBytes + len(targets)*8

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes+message.Len()))
	if disconnect {
		writer.WriteUint8s(def.DataBit | def.DisconnectBit)
	} else {
		writer.WriteUint8s(def.DataBit)
	}
	writer.WriteUint8s(route.Multicast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteUint16s(binary.BigEndian, uint16(len(targets)))
	writer.WriteInt64s(binary.BigEndian, targets...)

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodeMulticastReq 解码组播请求
// 协议：size + header + route + seq + session kind + count + targets + <message packet>
func DecodeMulticastReq(data []byte) (seq uint64, kind session.Kind, targets []int64, disconnect bool, message []byte, err error) {
	reader := buffer.NewReader(data)

	if _, err = reader.Seek(def.SizeBytes, io.SeekStart); err != nil {
		return
	}

	var k uint8

	if k, err = reader.ReadUint8(); err != nil {
		return
	} else {
		disconnect = k&def.DisconnectBit == def.DisconnectBit
	}

	if _, err = reader.Seek(def.RouteBytes, io.SeekCurrent); err != nil {
		return
	}

	if seq, err = reader.ReadUint64(binary.BigEndian); err != nil {
		return
	}

	if k, err = reader.ReadUint8(); err != nil {
		return
	} else {
		kind = session.Kind(k)
	}

	count, err := reader.ReadUint16(binary.BigEndian)
	if err != nil {
		return
	}

	if targets, err = reader.ReadInt64s(binary.BigEndian, int(count)); err != nil {
		return
	}

	message = data[multicastReqBytes+def.B64*int(count):]

	return
}

// EncodeMulticastRes 编码组播响应
// 协议：size + header + route + seq + code + [total]
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
// 协议：size + header + route + seq + code + [total]
func DecodeMulticastRes(data []byte) (code uint16, total uint64, err error) {
	if len(data) != multicastResBytes && len(data) != multicastResBytes-def.B64 {
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

	if code == codes.OK && len(data) == multicastResBytes {
		total, err = reader.ReadUint64(binary.BigEndian)
	}

	return
}
