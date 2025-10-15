package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	handshakeReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8
	handshakeResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeHandshakeReq 编码握手请求
// 协议：size + header + route + seq + ins kind + ins id
func EncodeHandshakeReq(seq uint64, insKind cluster.Kind, insID string) *buffer.NocopyBuffer {
	size := handshakeReqBytes + len(insID)

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Handshake)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(insKind))
	writer.WriteString(insID)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeHandshakeReq 解码握手请求
// 协议：size + header + route + seq + ins kind + ins id
func DecodeHandshakeReq(data []byte) (seq uint64, insKind cluster.Kind, insID string, err error) {
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
		insKind = cluster.Kind(k)
	}

	if insID, err = reader.ReadString(len(data) - handshakeReqBytes); err != nil {
		return
	}

	return
}

// EncodeHandshakeRes 编码握手响应
// 协议：size + header + route + seq + code
func EncodeHandshakeRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(handshakeResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(handshakeResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Handshake)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeHandshakeRes 解码握手响应
// 协议：size + header + route + seq + code
func DecodeHandshakeRes(data []byte) (code uint16, err error) {
	if len(data) != handshakeResBytes {
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
