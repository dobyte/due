package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

const (
	unbindReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B64
	unbindResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes
)

// EncodeUnbindReq encodes an unbind request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {uid}
func EncodeUnbindReq(seq uint64, uid int64) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(unbindReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unbindReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Unbind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteInt64s(binary.BigEndian, uid)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnbindReq decodes an unbind request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {uid}
func DecodeUnbindReq(buf buffer.Buffer) (uid int64, err error) {
	if buf.Len() != def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	uid = int64(binary.BigEndian.Uint64(buf.Bytes()))

	return
}

// EncodeUnbindRes encodes an unbind response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func EncodeUnbindRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(unbindResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(unbindResBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.Unbind)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeUnbindRes decodes an unbind response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code}
func DecodeUnbindRes(buf buffer.Buffer) (uint16, error) {
	if buf.Len() != def.CodeBytes {
		return 0, errors.ErrInvalidMessage
	}

	return binary.BigEndian.Uint16(buf.Bytes()), nil
}
