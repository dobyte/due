package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestGetIPReqRoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		encode       func(uint64, cluster.Session, int64) *buffer.NocopyBuffer
		wantExtended bool
		wantPayload  int
	}{
		{name: "legacy", encode: protocol.EncodeGetIPReq, wantExtended: false, wantPayload: def.B8 + def.B64},
		{name: "extended", encode: protocol.EncodeGetIPReqExt, wantExtended: true, wantPayload: def.B8 + def.B64 + def.B8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.encode(1, cluster.User, 3)
			defer req.Release()

			payload := req.Bytes()[def.MinFrameSize:]
			if len(payload) != tt.wantPayload {
				t.Fatalf("invalid payload length, expect: %d, actual: %d", tt.wantPayload, len(payload))
			}

			kind, target, extended, err := protocol.DecodeGetIPReq(buffer.NewBytes(payload))
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if kind != cluster.User || target != 3 || extended != tt.wantExtended {
				t.Fatalf("invalid result, expect: (%d, %d, %v), actual: (%d, %d, %v)",
					cluster.User, 3, tt.wantExtended, kind, target, extended)
			}
		})
	}
}

func TestDecodeGetIPReqInvalidLength(t *testing.T) {
	if _, _, _, err := protocol.DecodeGetIPReq(buffer.NewBytes([]byte{0, 0, 0})); err == nil {
		t.Fatal("expect an error for a request of an invalid length")
	}
}

func TestGetIPResRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		extended    bool
		code        uint16
		ip          string
		wantCode    uint16
		wantIP      string
		wantPayload int
	}{
		{name: "legacy ipv4", code: codes.OK, ip: "192.168.0.88", wantCode: codes.OK, wantIP: "192.168.0.88", wantPayload: def.CodeBytes + def.B32},
		{name: "legacy ipv4 mapped", code: codes.OK, ip: "::ffff:192.168.0.88", wantCode: codes.OK, wantIP: "192.168.0.88", wantPayload: def.CodeBytes + def.B32},
		{name: "legacy ipv6", code: codes.OK, ip: "2408:db8::1", wantCode: codes.OK, wantIP: "", wantPayload: def.CodeBytes},
		{name: "legacy empty ip", code: codes.OK, ip: "", wantCode: codes.OK, wantIP: "", wantPayload: def.CodeBytes},
		{name: "legacy invalid ip", code: codes.OK, ip: "invalid", wantCode: codes.OK, wantIP: "", wantPayload: def.CodeBytes},
		{name: "legacy error code", code: codes.NotFoundSession, ip: "192.168.0.88", wantCode: codes.NotFoundSession, wantIP: "", wantPayload: def.CodeBytes},
		{name: "extended ipv4", extended: true, code: codes.OK, ip: "192.168.0.88", wantCode: codes.OK, wantIP: "192.168.0.88", wantPayload: def.CodeBytes + def.B32},
		{name: "extended ipv4 mapped", extended: true, code: codes.OK, ip: "::ffff:192.168.0.88", wantCode: codes.OK, wantIP: "192.168.0.88", wantPayload: def.CodeBytes + def.B32},
		{name: "extended ipv6", extended: true, code: codes.OK, ip: "2408:db8::1", wantCode: codes.OK, wantIP: "2408:db8::1", wantPayload: def.CodeBytes + def.B128},
		{name: "extended empty ip", extended: true, code: codes.OK, ip: "", wantCode: codes.OK, wantIP: "", wantPayload: def.CodeBytes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encode := protocol.EncodeGetIPRes
			if tt.extended {
				encode = protocol.EncodeGetIPResExt
			}

			res := encode(1, tt.code, tt.ip)
			defer res.Release()

			payload := res.Bytes()[def.MinFrameSize:]
			if len(payload) != tt.wantPayload {
				t.Fatalf("invalid payload length, expect: %d, actual: %d", tt.wantPayload, len(payload))
			}

			code, ip, err := protocol.DecodeGetIPRes(buffer.NewBytes(payload))
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if code != tt.wantCode || ip != tt.wantIP {
				t.Fatalf("invalid result, expect: (%d, %q), actual: (%d, %q)", tt.wantCode, tt.wantIP, code, ip)
			}
		})
	}
}
