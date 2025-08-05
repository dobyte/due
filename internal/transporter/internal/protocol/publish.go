package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	publishReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8
	publishResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b64
)

// EncodePublishReq 编码发布频道消息请求
// 协议：size + header + route + seq + channel len + channel + <message packet>
func EncodePublishReq(seq uint64, channel string, message buffer.Buffer) buffer.Buffer {
	channelBytes := len([]byte(channel))
	size := publishReqBytes + channelBytes
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-defaultSizeBytes+message.Len()))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Publish)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(channelBytes))
	writer.WriteString(channel)
	buf.Mount(message)

	return buf
}

// DecodePublishReq 解码发布频道消息请求
// 协议：size + header + route + seq + channel len + channel + <message packet>
func DecodePublishReq(data []byte) (seq uint64, channel string, message []byte, err error) {
	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes, io.SeekStart); err != nil {
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
// 协议：size + header + route + seq + total
func EncodePublishRes(seq uint64, total uint64) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(publishResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(publishResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Publish)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint64s(binary.BigEndian, total)

	return buf
}

// DecodeMulticastRes 解码组播响应
// 协议：size + header + route + seq + code + [total]
func DecodePublishRes(data []byte) (total uint64, err error) {
	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+defaultSeqBytes, io.SeekStart); err != nil {
		return
	}

	total, err = reader.ReadUint64(binary.BigEndian)

	return
}
