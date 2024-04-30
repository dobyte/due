package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/stream/internal/route"
	"github.com/dobyte/due/v2/session"
	"io"
)

const (
	multicastReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 1 + 2
	multicastResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeMulticastReq 编码组播请求（最多组播65535个对象）
// 协议：size + header + route + seq + session kind + count + targets + <message packet>
func EncodeMulticastReq(seq uint64, kind session.Kind, targets []int64, message []byte) buffer.Buffer {
	size := multicastReqBytes + len(targets)*8
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-defaultSizeBytes+len(message)))
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

	message = data[multicastReqBytes+count:]

	return
}
