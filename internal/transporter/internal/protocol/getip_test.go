package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeGetIPReq(t *testing.T) {
	buffer := protocol.EncodeGetIPReq(1, cluster.User, 3)

	t.Log(buffer.Bytes())
}

func TestDecodeGetIPReq(t *testing.T) {
	req := protocol.EncodeGetIPReq(1, cluster.User, 3)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, target, err := protocol.DecodeGetIPReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("target: %v", target)
}

func TestGetIPResRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		code        uint16
		ip          string
		wantCode    uint16
		wantIP      string
		wantPayload int
	}{
		{name: "ipv4", code: codes.OK, ip: "192.168.0.88", wantCode: codes.OK, wantIP: "192.168.0.88", wantPayload: def.CodeBytes + def.B32},
		{name: "ipv4 mapped", code: codes.OK, ip: "::ffff:192.168.0.88", wantCode: codes.OK, wantIP: "192.168.0.88", wantPayload: def.CodeBytes + def.B32},
		{name: "ipv6", code: codes.OK, ip: "2408:db8::1", wantCode: codes.OK, wantIP: "2408:db8::1", wantPayload: def.CodeBytes + def.B128},
		{name: "empty ip", code: codes.OK, ip: "", wantCode: codes.OK, wantIP: "", wantPayload: def.CodeBytes},
		{name: "invalid ip", code: codes.OK, ip: "invalid", wantCode: codes.OK, wantIP: "", wantPayload: def.CodeBytes},
		{name: "error code", code: codes.NotFoundSession, ip: "192.168.0.88", wantCode: codes.NotFoundSession, wantIP: "", wantPayload: def.CodeBytes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := protocol.EncodeGetIPRes(1, tt.code, tt.ip)
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
