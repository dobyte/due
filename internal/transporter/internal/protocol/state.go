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
	getStateReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes
	getStateResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes + b8
	setStateReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8
	setStateResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeGetStateReq 编码获取状态请求
// 协议：size + header + route + seq
func EncodeGetStateReq(seq uint64) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(getStateReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(getStateReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.GetState)
	writer.WriteUint64s(binary.BigEndian, seq)

	return buf
}

// DecodeGetStateReq 解码获取状态请求
// 协议：size + header + route + seq
func DecodeGetStateReq(data []byte) (seq uint64, err error) {
	if len(data) != getStateReqBytes {
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

	return
}

// EncodeGetStateRes 编码获取状态响应
// 协议：size + header + route + seq + code + cluster state
func EncodeGetStateRes(seq uint64, code uint16, state cluster.State) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(getStateResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(getStateResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.GetState)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)
	writer.WriteUint8s(uint8(state))

	return buf
}

// DecodeGetStateRes 解码获取状态响应
// 协议：size + header + route + seq + code + cluster state
func DecodeGetStateRes(data []byte) (code uint16, state cluster.State, err error) {
	if len(data) != getStateResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+defaultSeqBytes, io.SeekStart); err != nil {
		return
	}

	if code, err = reader.ReadUint16(binary.BigEndian); err != nil {
		return
	}

	if status, e := reader.ReadUint8(); e != nil {
		err = e
	} else {
		state = cluster.State(status)
	}

	return
}

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

// DecodeSetStateReq 解码设置状态请求
// 协议：size + header + route + seq + cluster state
func DecodeSetStateReq(data []byte) (seq uint64, state cluster.State, err error) {
	if len(data) != setStateReqBytes {
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

	if status, e := reader.ReadUint8(); e != nil {
		err = e
	} else {
		state = cluster.State(status)
	}

	return
}

// EncodeSetStateRes 编码设置状态响应
// 协议：size + header + route + seq + code
func EncodeSetStateRes(seq uint64, code uint16) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(setStateReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(setStateReqBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.SetState)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

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
