package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/stream/internal/route"
	"io"
)

const (
	isOnlineReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 8
	isOnlineResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeIsOnlineReq 编码检测用户是否在线请求
// 协议：size + header + route + seq + uid
func EncodeIsOnlineReq(seq uint64, uid int64) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(isOnlineReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(isOnlineReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.IsOnline)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, uid)

	return buf
}

// DecodeIsOnlineReq 解码检测用户是否在线请求
// 协议：size + header + route + seq + uid
func DecodeIsOnlineReq(data []byte) (seq uint64, uid int64, err error) {
	if len(data) != isOnlineReqBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes, io.SeekStart); err != nil {
		return
	}

	if seq, err = reader.ReadUint64(binary.BigEndian); err != nil {
		return
	}

	if uid, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	return
}

// EncodeIsOnlineRes 编码检测用户是否在线响应
// 协议：size + header + route + seq + code
func EncodeIsOnlineRes(seq uint64, code int16) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(isOnlineResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(isOnlineResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.IsOnline)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt16s(binary.BigEndian, code)

	return buf
}

// DecodeIsOnlineRes 解码检测用户是否在线响应
// 协议：size + header + route + seq + code
func DecodeIsOnlineRes(data []byte) (code int16, err error) {
	if len(data) != isOnlineResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+defaultSeqBytes, io.SeekStart); err != nil {
		return
	}

	if code, err = reader.ReadInt16(binary.BigEndian); err != nil {
		return
	}

	return
}
