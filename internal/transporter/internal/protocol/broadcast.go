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
	broadcastReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8
	broadcastResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodeBroadcastReq 编码广播请求
// 协议：size + header + route + seq + session kind + <message packet>
func EncodeBroadcastReq(seq uint64, kind session.Kind, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(broadcastReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(broadcastReqBytes-def.SizeBytes+message.Len()))
	if disconnect {
		writer.WriteUint8s(def.DataBit | def.DisconnectBit)
	} else {
		writer.WriteUint8s(def.DataBit)
	}
	writer.WriteUint8s(route.Broadcast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodeBroadcastReq 解码广播请求
// 协议：size + header + route + seq + session kind + <message packet>
func DecodeBroadcastReq(data []byte) (seq uint64, kind session.Kind, disconnect bool, message []byte, err error) {
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

	message = data[broadcastReqBytes:]

	return
}

// EncodeBroadcastRes 编码广播响应
// 协议：size + header + route + seq + code + [total]
func EncodeBroadcastRes(seq uint64, code uint16, total ...uint64) *buffer.NocopyBuffer {
	size := broadcastResBytes - def.SizeBytes
	if code != codes.OK || len(total) == 0 || total[0] == 0 {
		size -= def.B64
	}

	writer := buffer.MallocWriter(broadcastResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Broadcast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(total) > 0 && total[0] != 0 {
		writer.WriteUint64s(binary.BigEndian, total[0])
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeBroadcastRes 解码广播响应
// 协议：size + header + route + seq + code + [total]
func DecodeBroadcastRes(data []byte) (code uint16, total uint64, err error) {
	if len(data) != broadcastResBytes && len(data) != broadcastResBytes-def.B64 {
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

	if code == codes.OK && len(data) == broadcastResBytes {
		total, err = reader.ReadUint64(binary.BigEndian)
	}

	return
}
