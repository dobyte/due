package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

const (
	pushReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64
	pushResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodePushReq 编码推送请求
// 协议：size + header + route + seq + session kind + target + <message packet>
func EncodePushReq(seq uint64, kind session.Kind, target int64, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(pushReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(pushReqBytes-def.SizeBytes+message.Len()))
	if disconnect {
		writer.WriteUint8s(def.DataBit | def.DisconnectBit)
	} else {
		writer.WriteUint8s(def.DataBit)
	}
	writer.WriteUint8s(route.Push)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodePushReq 解码推送消息
// 协议：size + header + route + seq + session kind + target + <message packet>
func DecodePushReq(data []byte) (seq uint64, kind session.Kind, target int64, disconnect bool, message []byte, err error) {
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

	if target, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	message = data[pushReqBytes:]

	return
}

// EncodePushRes 编码推送响应
// 协议：size + header + route + seq + code
func EncodePushRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(pushResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(pushResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Push)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodePushRes 解码推送响应
// 协议：size + header + route + seq + code
func DecodePushRes(data []byte) (code uint16, err error) {
	if len(data) != pushResBytes {
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
