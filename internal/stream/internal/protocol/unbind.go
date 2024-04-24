package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/stream/internal/route"
)

const (
	unbindReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 8
	unbindResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeUnbindReq 打包解绑请求
// 协议格式：size + header + route + seq + uid
func EncodeUnbindReq(seq uint64, uid int64) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(unbindReqBytes)
	writer.WriteInt32s(binary.BigEndian, int32(unbindReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Unbind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, uid)

	return buf
}
