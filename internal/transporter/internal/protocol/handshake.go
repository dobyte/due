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
	handshakeReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64
	handshakeResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeHandshakeReq encodes a handshake request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {ins kind + ins id + conn epoch}
func EncodeHandshakeReq(seq uint64, kind cluster.Kind, inst string, epoch uint64) buffer.Buffer {
	size := handshakeReqBytes + len(inst)
	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Handshake)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteString(inst)
	writer.WriteUint64s(binary.BigEndian, epoch)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeHandshakeReq decodes a handshake request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {ins kind + ins id + conn epoch}
func DecodeHandshakeReq(buf buffer.Buffer) (cluster.Kind, string, uint64, error) {
	if buf.Len() < def.B8+def.B64+1 {
		return 0, "", 0, errors.ErrInvalidMessage
	}

	data := buf.Bytes()
	kind := cluster.Kind(data[0])
	inst := string(data[def.B8 : len(data)-def.B64])
	epoch := binary.BigEndian.Uint64(data[len(data)-def.B64:])

	return kind, inst, epoch, nil
}

// EncodeHandshakeRes encodes a handshake response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func EncodeHandshakeRes(seq uint64, code uint16) buffer.Buffer {
	writer := buffer.MallocWriter(handshakeResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(handshakeResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Handshake)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeHandshakeRes decodes a handshake response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func DecodeHandshakeRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
