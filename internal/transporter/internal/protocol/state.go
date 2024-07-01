package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"io"
)

const (
	setStateReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8
	setStateResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeSetStateReq 编码设置状态请求
// 协议：size + header + route + seq + cluster state
func EncodeSetStateReq(seq uint64, state cluster.State) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(setStateReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(setStateReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.SetState)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(state))

	return buf
}

// DecodeSetStateRes 解码绑定响应
// 协议：size + header + route + seq + code
func DecodeSetStateRes(data []byte) (code uint16, err error) {
	if len(data) != setStateResBytes {
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
