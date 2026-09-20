package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	publishReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B8
	publishResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodePublishReq 编码发布频道消息请求
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{channel len + channel + disconnect + <message packet>}
func EncodePublishReq(seq uint64, channel string, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	channelBytes := len([]byte(channel))
	size := publishReqBytes + channelBytes

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes+message.Len()))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Publish)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(channelBytes))
	writer.WriteString(channel)
	writer.WriteBools(disconnect)

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodePublishReq 解码发布频道消息请求
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{channel len + channel + disconnect + <message packet>}
func DecodePublishReq(req *buffer.Bytes) (string, bool, *buffer.Bytes, error) {
	data := req.Bytes()

	if len(data) < def.B8 {
		return "", false, nil, errors.ErrInvalidMessage
	}

	channelBytes := int(data[0])
	offset := def.B8 + channelBytes + def.B8
	if len(data) < offset {
		return "", false, nil, errors.ErrInvalidMessage
	}

	channel := string(data[def.B8 : def.B8+channelBytes])
	disconnect := data[def.B8+channelBytes] == 1

	req.Slide(offset)

	return channel, disconnect, req, nil
}

// EncodePublishRes 编码发布频道消息响应
// 注意：buf 包含全段协议
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [total]}
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

// DecodePublishRes 解码发布频道消息响应
// 注意：buf 仅包含私有段
// 协议：公共段：{size + header + route + seq} + 私有段：{code + [total]}
func DecodePublishRes(buf buffer.Buffer) (code uint16, total uint64, err error) {
	if buf.Len() != def.CodeBytes && buf.Len() != def.CodeBytes+def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	code = binary.BigEndian.Uint16(data[:def.CodeBytes])

	if code == codes.OK && buf.Len() == def.CodeBytes+def.B64 {
		total = binary.BigEndian.Uint64(data[def.CodeBytes:])
	}

	return
}
