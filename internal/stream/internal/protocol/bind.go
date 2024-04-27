package protocol

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/stream/internal/route"
	"io"
)

const (
	bindReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 8 + 8
	bindResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeBindReq 编码绑定请求
// 协议：size + header + route + seq + cid + uid
func EncodeBindReq(seq uint64, cid, uid int64) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(bindReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(bindReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Bind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, cid, uid)

	return buf
}

// DecodeBindReq 解码绑定请求
// 协议：size + header + route + seq + cid + uid
func DecodeBindReq(data []byte) (seq uint64, cid, uid int64, err error) {
	if len(data) != bindReqBytes {
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

	if cid, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	if uid, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	return
}

// EncodeBindRes 编码绑定响应
// 协议：size + header + route + seq + code
func EncodeBindRes(seq uint64, code int16) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(bindResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(bindResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Bind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt16s(binary.BigEndian, code)

	return buf
}

// DecodeBindRes 解码绑定响应
// 协议：size + header + route + seq + code
func DecodeBindRes(data []byte) (code int16, err error) {
	if len(data) != bindResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := bytes.NewReader(data)

	if _, err = reader.Seek(-defaultCodeBytes, io.SeekEnd); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &code); err != nil {
		return
	}

	return
}
