package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
	"io"
)

const (
	multicastReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8 + b16
	multicastResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes + b64
)

// EncodeMulticastReq 编码组播请求（最多组播65535个对象）
// 协议：size + header + route + seq + session kind + count + targets + <message packet>
func EncodeMulticastReq(seq uint64, kind session.Kind, targets []int64, message buffer.Buffer) buffer.Buffer {
	size := multicastReqBytes + len(targets)*8
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-defaultSizeBytes+message.Len()))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Multicast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteUint16s(binary.BigEndian, uint16(len(targets)))
	writer.WriteInt64s(binary.BigEndian, targets...)
	buf.Mount(message)

	return buf
}

// DecodeMulticastReq 解码组播请求
// 协议：size + header + route + seq + session kind + count + targets + <message packet>
func DecodeMulticastReq(data []byte) (seq uint64, kind session.Kind, targets []int64, message []byte, err error) {
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

	message = data[multicastReqBytes+8*count:]

	return
}

// EncodeMulticastRes 编码组播响应
// 协议：size + header + route + seq + code + [total]
func EncodeMulticastRes(seq uint64, code uint16, total ...uint64) buffer.Buffer {
	size := multicastResBytes - defaultSizeBytes
	if code != codes.OK || len(total) == 0 || total[0] == 0 {
		size -= b64
	}

	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(multicastResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Multicast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(total) > 0 && total[0] != 0 {
		writer.WriteUint64s(binary.BigEndian, total[0])
	}

	return buf
}

// DecodeMulticastRes 解码组播响应
// 协议：size + header + route + seq + code + [total]
func DecodeMulticastRes(data []byte) (code uint16, total uint64, err error) {
	if len(data) != multicastResBytes && len(data) != multicastResBytes-b64 {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+defaultSeqBytes, io.SeekStart); err != nil {
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
