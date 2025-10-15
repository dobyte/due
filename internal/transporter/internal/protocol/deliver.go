package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	deliverReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b64 + b64
	deliverResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeDeliverReq 编码投递消息请求
// 协议：size + header + route + seq + cid + uid + <message packet>
func EncodeDeliverReq(seq uint64, cid int64, uid int64, buf buffer.Buffer) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(deliverReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(deliverReqBytes-defaultSizeBytes+buf.Len()))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Deliver)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, cid, uid)

	return buffer.NewNocopyBuffer(writer, buf)
}

// DecodeDeliverReq 解码投递消息请求
func DecodeDeliverReq(data []byte) (seq uint64, cid int64, uid int64, message []byte, err error) {
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

	message = data[deliverReqBytes:]

	return
}

// EncodeDeliverRes 编码投递消息响应
// 协议：size + header + route + seq + code
func EncodeDeliverRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(deliverResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(deliverResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Deliver)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeDeliverRes 解码投递消息响应
// 协议：size + header + route + seq + code
func DecodeDeliverRes(data []byte) (code uint16, err error) {
	if len(data) != deliverResBytes {
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
