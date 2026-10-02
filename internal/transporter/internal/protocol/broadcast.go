package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

const (
	broadcastReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B8
	broadcastResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B64
)

// EncodeBroadcastReq encodes a broadcast request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + disconnect + <message packet>}
func EncodeBroadcastReq(seq uint64, kind session.Kind, disconnect bool, message buffer.Buffer) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(broadcastReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(broadcastReqBytes-def.SizeBytes+message.Len()))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Broadcast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteBools(disconnect)

	return buffer.NewNocopyBuffer(writer, message)
}

// DecodeBroadcastReq decodes a broadcast request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + disconnect + <message packet>}
func DecodeBroadcastReq(req *buffer.Bytes) (session.Kind, bool, *buffer.Bytes, error) {
	if req.Len() < def.B8+def.B8 {
		return 0, false, nil, errors.ErrInvalidMessage
	}

	data := req.Bytes()
	kind := session.Kind(data[0])
	disconnect := data[def.B8] == 1

	req.Slide(def.B8 + def.B8)

	return kind, disconnect, req, nil
}

// EncodeBroadcastRes encodes a broadcast response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code + [total]}
func EncodeBroadcastRes(seq uint64, code uint16, total ...uint64) *buffer.NocopyBuffer {
	size := broadcastResBytes - def.SizeBytes
	if code != codes.OK || len(total) == 0 || total[0] == 0 {
		size -= def.B64
	}

	writer := buffer.MallocWriter(broadcastResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Broadcast)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	if code == codes.OK && len(total) > 0 && total[0] != 0 {
		writer.WriteUint64s(binary.BigEndian, total[0])
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeBroadcastRes decodes a broadcast response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code + [total]}
func DecodeBroadcastRes(buf buffer.Buffer) (code uint16, total uint64, err error) {
	if buf.Len() != def.CodeBytes && buf.Len() != def.CodeBytes+def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	code = binary.BigEndian.Uint16(data[:def.CodeBytes])

	if code == codes.OK && buf.Len() == def.CodeBytes+def.B64 {
		total = binary.BigEndian.Uint64(data[def.CodeBytes:])
	}

	return
}
