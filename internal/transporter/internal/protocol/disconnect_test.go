package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
)

func TestEncodeDisconnectReq(t *testing.T) {
	buffer := protocol.EncodeDisconnectReq(1, session.User, 3, true)

	t.Log(buffer.Bytes())
}

func TestDecodeDisconnectReq(t *testing.T) {
	req := protocol.EncodeDisconnectReq(1, session.User, 3, false)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, target, force, err := protocol.DecodeDisconnectReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("target: %v", target)
	t.Logf("force: %v", force)
}

func TestEncodeDisconnectRes(t *testing.T) {
	buffer := protocol.EncodeDisconnectRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeDisconnectRes(t *testing.T) {
	req := protocol.EncodeDisconnectRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeDisconnectRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
