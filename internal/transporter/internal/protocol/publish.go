package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	publishReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8
	publishResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodePublishReq 编码发布频道消息请求
// 协议：size + header + route + seq + channel len + channel + <message packet>
func EncodePublishReq(seq uint64, channel string, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	channelBytes := len([]byte(channel))
	size := publishReqBytes + channelBytes

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes+message.Len()))
	if disconnect {
		writer.WriteUint8s(def.DataBit | def.DisconnectBit)
	} else {
		writer.WriteUint8s(def.DataBit)
	}
	writer.WriteUint8s(route.Publish)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(channelBytes))
	writer.WriteString(channel)

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodePublishReq 解码发布频道消息请求
// 协议：size + header + route + seq + channel len + channel + <message packet>
func DecodePublishReq(data []byte) (seq uint64, channel string, disconnect bool, message []byte, err error) {
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

	var channelBytes uint8

	if channelBytes, err = reader.ReadUint8(); err != nil {
		return
	}

	if channel, err = reader.ReadString(int(channelBytes)); err != nil {
		return
	}

	message = data[publishReqBytes+channelBytes:]

	return
}

// EncodePublishRes 编码发布频道消息响应
// 协议：size + header + route + seq + code + [total]
func EncodePublishRes(seq uint64, code uint16, total ...uint64) *buffer.NocopyBuffer {
	size := publishResBytes - def.SizeBytes
	if code != codes.OK || len(total) == 0 || total[0] == 0 {
		size -= def.B64
	}

	writer := buffer.MallocWriter(publishResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Publish)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(total) > 0 && total[0] != 0 {
		writer.WriteUint64s(binary.BigEndian, total[0])
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeMulticastRes 解码组播响应
// 协议：size + header + route + seq + code + [total]
func DecodePublishRes(data []byte) (code uint16, total uint64, err error) {
	if len(data) != publishResBytes && len(data) != publishResBytes-def.B64 {
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

	if code == codes.OK && len(data) == publishResBytes {
		total, err = reader.ReadUint64(binary.BigEndian)
	}

	return
}
