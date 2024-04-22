package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/stream/internal/route"
)

const (
	bindReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 8 + 8
	bindResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeBindReq 编码绑定请求
// 协议格式：size + header + route + seq + cid + uid
func EncodeBindReq(seq uint64, cid, uid int64) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(bindReqBytes)
	writer.WriteInt32s(binary.BigEndian, int32(bindReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Bind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, cid, uid)

	return buf
}

func DecodeBindReq() {

}

// EncodeBindRes 编码绑定响应
// size + header + route + seq + code
func EncodeBindRes(seq uint64, code int16) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(bindResBytes)
	writer.WriteInt32s(binary.BigEndian, int32(bindResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Bind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt16s(binary.BigEndian, code)

	return buf
}
