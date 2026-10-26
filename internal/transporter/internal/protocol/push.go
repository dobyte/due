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
	pushReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64 + def.B8
	pushResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodePushReq encodes a push request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + target + disconnect + <message packet>}
func EncodePushReq(seq uint64, kind cluster.Session, target int64, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(pushReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(pushReqBytes-def.SizeBytes+message.Len()))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Push)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)
	writer.WriteBools(disconnect)

	return buffer.NewNocopyBuffer(writer, message)
}

// PushReqHeadroom returns the headroom required to prepend a push request header in place.
func PushReqHeadroom() int {
	return pushReqBytes
}

// EncodePushReqBuffer encodes a push request into the headroom reserved by message. It returns
// message itself on success and falls back to EncodePushReq when the headroom is insufficient.
func EncodePushReqBuffer(seq uint64, kind cluster.Session, target int64, disconnect bool, message *buffer.Bytes) buffer.Buffer {
	payload := message.Len()

	header := message.Prepend(pushReqBytes)
	if header == nil {
		return EncodePushReq(seq, kind, target, disconnect, message)
	}

	binary.BigEndian.PutUint32(header, uint32(pushReqBytes-def.SizeBytes+payload))

	offset := def.SizeBytes
	header[offset] = def.DataBit
	offset += def.HeaderBytes
	header[offset] = route.Push
	offset += def.RouteBytes
	binary.BigEndian.PutUint64(header[offset:], seq)
	offset += def.SeqBytes
	header[offset] = uint8(kind)
	offset += def.B8
	binary.BigEndian.PutUint64(header[offset:], uint64(target))
	offset += def.B64
	if disconnect {
		header[offset] = 1
	} else {
		header[offset] = 0
	}

	return message
}

// DecodePushReq decodes a push request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + target + disconnect + <message packet>}
func DecodePushReq(req *buffer.Bytes) (cluster.Session, int64, bool, *buffer.Bytes, error) {
	if req.Len() < def.B8+def.B64+def.B8 {
		return 0, 0, false, nil, errors.ErrInvalidMessage
	}

	data := req.Bytes()
	kind := cluster.Session(data[0])
	target := int64(binary.BigEndian.Uint64(data[def.B8 : def.B8+def.B64]))
	disconnect := data[def.B8+def.B64] == 1

	req.Slide(def.B8 + def.B64 + def.B8)

	return kind, target, disconnect, req, nil
}

// EncodePushRes encodes a push response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func EncodePushRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(pushResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(pushResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Push)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodePushRes decodes a push response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func DecodePushRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
