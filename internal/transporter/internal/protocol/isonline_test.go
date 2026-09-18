package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
)

func TestDecodeIsOnlineReq(t *testing.T) {
	req := protocol.EncodeIsOnlineReq(1, session.User, 1)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, target, err := protocol.DecodeIsOnlineReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("target: %v", target)
}

func TestDecodeIsOnlineRes(t *testing.T) {
	req := protocol.EncodeIsOnlineRes(1, codes.NotFoundSession, false)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, isOnline, err := protocol.DecodeIsOnlineRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("isOnline: %v", isOnline)
}
