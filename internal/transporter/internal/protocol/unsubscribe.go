package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

const (
	unsubscribeReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B16
	unsubscribeResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeUnsubscribeReq encodes an unsubscribe request (at most 65535 targets per call).
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + count + targets + channel}
func EncodeUnsubscribeReq(seq uint64, kind session.Kind, targets []int64, channel string) *buffer.NocopyBuffer {
	size := unsubscribeReqBytes + len(targets)*8 + len([]byte(channel))

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Unsubscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteUint16s(binary.BigEndian, uint16(len(targets)))
	writer.WriteInt64s(binary.BigEndian, targets...)
	writer.WriteString(channel)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnsubscribeReq decodes an unsubscribe request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + count + targets + channel}
func DecodeUnsubscribeReq(buf buffer.Buffer) (kind session.Kind, targets []int64, channel string, err error) {
	data := buf.Bytes()

	if len(data) < def.B8+def.B16 {
		err = errors.ErrInvalidMessage
		return
	}

	kind = session.Kind(data[0])
	count := int(binary.BigEndian.Uint16(data[def.B8 : def.B8+def.B16]))

	offset := def.B8 + def.B16 + count*def.B64
	if len(data) < offset {
		err = errors.ErrInvalidMessage
		return
	}

	targets = make([]int64, count)
	for i := range count {
		start := def.B8 + def.B16 + i*def.B64
		targets[i] = int64(binary.BigEndian.Uint64(data[start : start+def.B64]))
	}

	channel = string(data[offset:])

	return
}

// EncodeUnsubscribeRes encodes an unsubscribe response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func EncodeUnsubscribeRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(unsubscribeResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unsubscribeResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Unsubscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnsubscribeRes decodes an unsubscribe response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func DecodeUnsubscribeRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
