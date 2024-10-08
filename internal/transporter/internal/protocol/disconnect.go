package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
	"io"
)

const (
	disconnectReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8 + b64 + b8
	disconnectResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeDisconnectReq 编码断连请求
// 协议：size + header + route + seq + session kind + target + force
func EncodeDisconnectReq(seq uint64, kind session.Kind, target int64, force bool) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(disconnectReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(disconnectReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Disconnect)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)
	writer.WriteBools(force)

	return buf
}

// DecodeDisconnectReq 解码端连请求
// 协议：size + header + route + seq + session kind + target + force
func DecodeDisconnectReq(data []byte) (seq uint64, kind session.Kind, target int64, force bool, err error) {
	if len(data) != disconnectReqBytes {
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

	var k uint8
	if k, err = reader.ReadUint8(); err != nil {
		return
	} else {
		kind = session.Kind(k)
	}

	if target, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	if force, err = reader.ReadBool(); err != nil {
		return
	}

	return
}

// EncodeDisconnectRes 编码断连响应
// 协议：size + header + route + seq + code
func EncodeDisconnectRes(seq uint64, code uint16) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(disconnectResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(disconnectResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Disconnect)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buf
}

// DecodeDisconnectRes 解码断连响应
// 协议：size + header + route + seq + code
func DecodeDisconnectRes(data []byte) (code uint16, err error) {
	if len(data) != bindResBytes {
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
