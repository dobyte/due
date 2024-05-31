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
	triggerReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8 + b64 + b64
	triggerResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeTriggerReq 编码触发事件请求
// 协议：size + header + route + seq + event + cid + [uid]
func EncodeTriggerReq(seq uint64, event cluster.Event, cid int64, uid ...int64) buffer.Buffer {
	size := triggerReqBytes - defaultSizeBytes
	if len(uid) == 0 || uid[0] == 0 {
		size -= b64
	}

	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(triggerReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Trigger)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(event))
	writer.WriteInt64s(binary.BigEndian, cid)

	if len(uid) > 0 && uid[0] != 0 {
		writer.WriteInt64s(binary.BigEndian, uid[0])
	}

	return buf
}

// DecodeTriggerReq 解码触发事件请求
// 协议：size + header + route + seq + event + cid + [uid]
func DecodeTriggerReq(data []byte) (seq uint64, event cluster.Event, cid int64, uid int64, err error) {
	if len(data) != triggerReqBytes && len(data) != triggerReqBytes-b64 {
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

	var evt uint8
	if evt, err = reader.ReadUint8(); err != nil {
		return
	} else {
		event = cluster.Event(evt)
	}

	if cid, err = reader.ReadInt64(binary.BigEndian); err != nil {
		return
	}

	if len(data) == triggerReqBytes {
		uid, err = reader.ReadInt64(binary.BigEndian)
	}

	return
}

// EncodeTriggerRes 编码触发事件响应
// 协议：size + header + route + seq + code
func EncodeTriggerRes(seq uint64, code uint16) buffer.Buffer {
	buf := buffer.NewNocopyBuffer()
	writer := buf.Malloc(triggerResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(triggerResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Trigger)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buf
}

// DecodeTriggerRes 解码触发事件响应
// 协议：size + header + route + seq + code
func DecodeTriggerRes(data []byte) (code uint16, err error) {
	if len(data) != triggerResBytes {
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
