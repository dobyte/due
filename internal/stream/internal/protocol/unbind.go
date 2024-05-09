package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/stream/internal/route"
	"io"
)

const (
	unbindReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b64
	unbindResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeUnbindReq 编码解绑请求
// 协议：size + header + route + seq + uid
func EncodeUnbindReq(seq uint64, uid int64) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(unbindReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unbindReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Unbind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, uid)

	return buf
}

// DecodeUnbindReq 解码解绑请求
// 协议：size + header + route + seq + uid
func DecodeUnbindReq(data []byte) (seq uint64, uid int64, err error) {
	if len(data) != unbindReqBytes {
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

// EncodeUnbindRes 编码解绑响应
// 协议：size + header + route + seq + code
func EncodeUnbindRes(seq uint64, code uint16) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(unbindResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unbindResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Unbind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buf
}

// DecodeUnbindRes 解码解绑响应
// 协议：size + header + route + seq + code
func DecodeUnbindRes(data []byte) (code uint16, err error) {
	if len(data) != unbindResBytes {
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
