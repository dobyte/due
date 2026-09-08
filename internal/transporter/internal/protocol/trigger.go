package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	triggerReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64 + def.B64
	triggerResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeTriggerReq 编码触发事件请求
// 协议：size + header + route + seq + event + cid + [uid]
func EncodeTriggerReq(seq uint64, event cluster.Event, cid int64, uid ...int64) *buffer.NocopyBuffer {
	size := triggerReqBytes - def.SizeBytes
	if len(uid) == 0 || uid[0] == 0 {
		size -= def.B64
	}

	writer := buffer.MallocWriter(triggerReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Trigger)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(event))
	writer.WriteInt64s(binary.BigEndian, cid)

	if len(uid) > 0 && uid[0] != 0 {
		writer.WriteInt64s(binary.BigEndian, uid[0])
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeTriggerReq 解码触发事件请求
// 协议：size + header + route + seq + event + cid + [uid]
func DecodeTriggerReq(data []byte) (seq uint64, event cluster.Event, cid int64, uid int64, err error) {
	if len(data) != triggerReqBytes && len(data) != triggerReqBytes-def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(def.SizeBytes+def.HeaderBytes+def.RouteBytes, io.SeekStart); err != nil {
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
func EncodeTriggerRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(triggerResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(triggerResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Trigger)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeTriggerRes 解码触发事件响应
// 协议：size + header + route + seq + code
func DecodeTriggerRes(data []byte) (code uint16, err error) {
	if len(data) != triggerResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(-def.CodeBytes, io.SeekEnd); err != nil {
		return
	}

	if code, err = reader.ReadUint16(binary.BigEndian); err != nil {
		return
	}

	return
}
