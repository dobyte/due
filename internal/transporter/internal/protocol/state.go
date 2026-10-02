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
	getStateReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes
	getStateResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B8
	setStateReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8
	setStateResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeGetStateReq encodes a get-state request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq}
func EncodeGetStateReq(seq uint64) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(getStateReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(getStateReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.GetState)
	writer.WriteUint64s(binary.BigEndian, seq)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeGetStateReq decodes a get-state request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq}
func DecodeGetStateReq(buf buffer.Buffer) error {
	if buf.Len() != 0 {
		return errors.ErrInvalidMessage
	}

	return nil
}

// EncodeGetStateRes encodes a get-state response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code + cluster state}
func EncodeGetStateRes(seq uint64, code uint16, state cluster.State) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(getStateResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(getStateResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.GetState)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)
	writer.WriteUint8s(uint8(state))

	return buffer.NewNocopyBuffer(writer)
}

// DecodeGetStateRes decodes a get-state response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code + cluster state}
func DecodeGetStateRes(buf buffer.Buffer) (code uint16, state cluster.State, err error) {
	if buf.Len() != def.CodeBytes+def.B8 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	code = binary.BigEndian.Uint16(data[:def.CodeBytes])
	state = cluster.State(data[def.CodeBytes])

	return
}

// EncodeSetStateReq encodes a set-state request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {cluster state}
func EncodeSetStateReq(seq uint64, state cluster.State) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(setStateReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(setStateReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.SetState)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(state))

	return buffer.NewNocopyBuffer(writer)
}

// DecodeSetStateReq decodes a set-state request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {cluster state}
func DecodeSetStateReq(buf buffer.Buffer) (state cluster.State, err error) {
	if buf.Len() != def.B8 {
		err = errors.ErrInvalidMessage
		return
	}

	state = cluster.State(buf.Bytes()[0])

	return
}

// EncodeSetStateRes encodes a set-state response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func EncodeSetStateRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(setStateResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(setStateResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.SetState)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeSetStateRes decodes a set-state response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func DecodeSetStateRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
