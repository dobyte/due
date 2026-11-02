package protocol

import (
	"encoding/binary"
	"net"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/utils/xnet"
)

const (
	getIPReqBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64
	getIPResBytes = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B128
)

// EncodeGetIPReq encodes a get-IP request.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + target}
func EncodeGetIPReq(seq uint64, kind cluster.Session, target int64) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(getIPReqBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(getIPReqBytes-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.GetIP)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeGetIPReq decodes a get-IP request.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + target}
func DecodeGetIPReq(buf buffer.Buffer) (kind cluster.Session, target int64, err error) {
	if buf.Len() != def.B8+def.B64 {
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	kind = cluster.Session(data[0])
	target = int64(binary.BigEndian.Uint64(data[def.B8:]))

	return
}

// EncodeGetIPRes encodes a get-IP response.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code + [ip]}
//
// The IP is encoded as 4 bytes for IPv4 and 16 bytes for IPv6, and omitted when the address is
// absent or cannot be parsed.
func EncodeGetIPRes(seq uint64, code uint16, ip string) *buffer.NocopyBuffer {
	var (
		size    = getIPResBytes - def.SizeBytes
		ipBytes int
	)

	if code == codes.OK && ip != "" {
		if parsed := net.ParseIP(ip); parsed != nil {
			if parsed.To4() != nil {
				ipBytes = def.B32
			} else {
				ipBytes = def.B128
			}
		}
	}

	size -= def.B128 - ipBytes

	writer := buffer.MallocWriter(getIPResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(size))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.GetIP)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	switch ipBytes {
	case def.B32:
		writer.WriteUint32s(binary.BigEndian, xnet.IP2Long(ip))
	case def.B128:
		raw := xnet.IP2Bytes(ip)
		writer.WriteBytes(raw[:]...)
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeGetIPRes decodes a get-IP response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} + private section: {code + [ip]}
func DecodeGetIPRes(buf buffer.Buffer) (code uint16, ip string, err error) {
	switch buf.Len() {
	case def.CodeBytes, def.CodeBytes + def.B32, def.CodeBytes + def.B128:
	default:
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	code = binary.BigEndian.Uint16(data[:def.CodeBytes])

	if code == codes.OK && buf.Len() > def.CodeBytes {
		if buf.Len() == def.CodeBytes+def.B32 {
			ip = xnet.Long2IP(binary.BigEndian.Uint32(data[def.CodeBytes:]))
		} else {
			var raw [16]byte
			copy(raw[:], data[def.CodeBytes:])
			ip = xnet.Bytes2IP(raw)
		}
	}

	return
}
