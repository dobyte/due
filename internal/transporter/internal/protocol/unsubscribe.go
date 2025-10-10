package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

const (
	unsubscribeReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8 + b16
	unsubscribeResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeUnsubscribeReq 编码取消订阅频道请求（单次最多取消订阅65535个对象）
// 协议：size + header + route + seq + session kind + count + targets + channel
func EncodeUnsubscribeReq(seq uint64, kind session.Kind, targets []int64, channel string) buffer.Buffer {
	size := unsubscribeReqBytes + len(targets)*8 + len([]byte(channel))

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Unsubscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteUint16s(binary.BigEndian, uint16(len(targets)))
	writer.WriteInt64s(binary.BigEndian, targets...)
	writer.WriteString(channel)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnsubscribeReq 解码取消订阅频道请求
// 协议：size + header + route + seq + session kind + count + targets + channel
func DecodeUnsubscribeReq(data []byte) (seq uint64, kind session.Kind, targets []int64, channel string, err error) {
	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes, io.SeekStart); err != nil {
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

	count, err := reader.ReadUint16(binary.BigEndian)
	if err != nil {
		return
	}

	if targets, err = reader.ReadInt64s(binary.BigEndian, int(count)); err != nil {
		return
	}

	channel = string(data[unsubscribeReqBytes+8*count:])

	return
}

// EncodeUnsubscribeRes 编码取消订阅频道响应
// 协议：size + header + route + seq + code
func EncodeUnsubscribeRes(seq uint64, code uint16) buffer.Buffer {
	writer := buffer.MallocWriter(unsubscribeResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unsubscribeResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Unsubscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnsubscribeRes 解码取消订阅频道响应
// 协议：size + header + route + seq + code
func DecodeUnsubscribeRes(data []byte) (code uint16, err error) {
	if len(data) != unsubscribeResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(-defaultCodeBytes, io.SeekEnd); err != nil {
		return
	}

	if code, err = reader.ReadUint16(binary.BigEndian); err != nil {
		return
	}

	return
}
