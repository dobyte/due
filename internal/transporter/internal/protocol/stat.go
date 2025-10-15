package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

const (
	statReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8
	statResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes + b64
)

// EncodeStatReq 编码统计在线人数请求
// 协议：size + header + route + seq + session kind
func EncodeStatReq(seq uint64, kind session.Kind) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(statReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(statReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Stat)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))

	return buffer.NewNocopyBuffer(writer)
}

// DecodeStatReq 解码统计在线人数请求
// 协议：size + header + route + seq + session kind
func DecodeStatReq(data []byte) (seq uint64, kind session.Kind, err error) {
	if len(data) != statReqBytes {
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

	if k, err = reader.ReadUint8(); err == nil {
		kind = session.Kind(k)
	}

	return
}

// EncodeStatRes 编码统计在线人数响应
// 协议：size + header + route + seq + code + [total]
func EncodeStatRes(seq uint64, code uint16, total ...uint64) *buffer.NocopyBuffer {
	size := statResBytes - defaultSizeBytes
	if code != codes.OK || len(total) == 0 || total[0] == 0 {
		size -= b64
	}

	writer := buffer.MallocWriter(statResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Stat)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(total) > 0 && total[0] != 0 {
		writer.WriteUint64s(binary.BigEndian, total[0])
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeStatRes 解码统计在线人数响应
// 协议：size + header + route + seq + code + [total]
func DecodeStatRes(data []byte) (code uint16, total uint64, err error) {
	if len(data) != statResBytes && len(data) != statResBytes-8 {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+defaultSeqBytes, io.SeekStart); err != nil {
		return
	}

	if len(data) == statResBytes {
		total, err = reader.ReadUint64(binary.BigEndian)
	}

	return
}
