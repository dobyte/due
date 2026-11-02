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
	getIPReqBytes    = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.B8 + def.B64
	getIPReqExtBytes = getIPReqBytes + def.B8
	getIPResBytes    = def.SizeBytes + def.HeaderBytes + def.RouteBytes + def.SeqBytes + def.CodeBytes + def.B128
)

// EncodeGetIPReq encodes a get-IP request in the legacy format.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {session kind + target}
func EncodeGetIPReq(seq uint64, kind cluster.Session, target int64) *buffer.NocopyBuffer {
	return encodeGetIPReq(seq, kind, target, false)
}

// EncodeGetIPReqExt encodes a get-IP request that accepts an IPv6 response.
//
// The extended format appends a flag byte to the legacy request, so it must only be sent after the
// peer has reported support through codes.IPv6Unsupported. Peers running the legacy protocol
// reject a request of an unexpected length.
func EncodeGetIPReqExt(seq uint64, kind cluster.Session, target int64) *buffer.NocopyBuffer {
	return encodeGetIPReq(seq, kind, target, true)
}

func encodeGetIPReq(seq uint64, kind cluster.Session, target int64, extended bool) *buffer.NocopyBuffer {
	size := getIPReqBytes
	if extended {
		size = getIPReqExtBytes
	}

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-def.SizeBytes))
	writer.WriteUint8s(def.DataBit)
	writer.WriteUint8s(route.GetIP)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteInt64s(binary.BigEndian, target)

	if extended {
		writer.WriteUint8s(1)
	}

	return buffer.NewNocopyBuffer(writer)
}

// DecodeGetIPReq decodes a get-IP request and reports whether the sender accepts an IPv6 response.
// Note that buf contains only the private section.
// Protocol: public section: {size + header + route + seq} +
// private section: {session kind + target + [extended]}
func DecodeGetIPReq(buf buffer.Buffer) (kind cluster.Session, target int64, extended bool, err error) {
	switch buf.Len() {
	case def.B8 + def.B64, def.B8 + def.B64 + def.B8:
	default:
		err = errors.ErrInvalidMessage
		return
	}

	data := buf.Bytes()
	kind = cluster.Session(data[0])
	target = int64(binary.BigEndian.Uint64(data[def.B8:]))
	extended = buf.Len() > def.B8+def.B64 && data[def.B8+def.B64] != 0

	return
}

// EncodeGetIPRes encodes a get-IP response in the legacy format.
// Note that buf contains the full protocol.
// Protocol: public section: {size + header + route + seq} + private section: {code + [ip]}
//
// The legacy format carries an IPv4 address in 4 bytes and omits the IP section for an address it
// cannot represent, such as an IPv6 address.
func EncodeGetIPRes(seq uint64, code uint16, ip string) *buffer.NocopyBuffer {
	return encodeGetIPRes(seq, code, ip, false)
}

// EncodeGetIPResExt encodes a get-IP response that can carry an IPv6 address.
//
// The extended format encodes an IPv4 address in 4 bytes and an IPv6 address in 16 bytes, so IPv4
// responses keep the legacy layout.
func EncodeGetIPResExt(seq uint64, code uint16, ip string) *buffer.NocopyBuffer {
	return encodeGetIPRes(seq, code, ip, true)
}

func encodeGetIPRes(seq uint64, code uint16, ip string, extended bool) *buffer.NocopyBuffer {
	var (
		size    = getIPResBytes - def.SizeBytes
		ipBytes int
	)

	if code == codes.OK && ip != "" {
		if parsed := net.ParseIP(ip); parsed != nil {
			if parsed.To4() != nil {
				ipBytes = def.B32
			} else if extended {
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
