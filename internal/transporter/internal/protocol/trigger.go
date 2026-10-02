package protocol

import (
	"encoding/binary"

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

// EncodeTriggerReq encodes a trigger request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {event + cid + [uid]}
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

// DecodeTriggerReq decodes a trigger request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {event + cid + [uid]}
func DecodeTriggerReq(buf *buffer.Bytes) (event cluster.Event, cid int64, uid int64, err error) {
	if buf.Len() != def.B8+def.B64 && buf.Len() != def.B8+def.B64+def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	event = cluster.Event(data[0])
	cid = int64(binary.BigEndian.Uint64(data[def.B8 : def.B8+def.B64]))

	if buf.Len() == def.B8+def.B64+def.B64 {
		uid = int64(binary.BigEndian.Uint64(data[def.B8+def.B64:]))
	}

	return
}

// EncodeTriggerRes encodes a trigger response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func EncodeTriggerRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(triggerResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(triggerResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Trigger)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeTriggerRes decodes a trigger response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func DecodeTriggerRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
