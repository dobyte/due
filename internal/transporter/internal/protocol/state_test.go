package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestDecodeGetStateReq(t *testing.T) {
	req := protocol.EncodeGetStateReq(1)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	if err := protocol.DecodeGetStateReq(buf); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeGetStateRes(t *testing.T) {
	req := protocol.EncodeGetStateRes(1, codes.OK, cluster.Work)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, state, err := protocol.DecodeGetStateRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("state: %v", state)
}

func TestDecodeSetStateReq(t *testing.T) {
	req := protocol.EncodeSetStateReq(1, cluster.Shut)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	state, err := protocol.DecodeSetStateReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("state: %v", state)
}

func TestDecodeSetStateRes(t *testing.T) {
	req := protocol.EncodeSetStateRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeSetStateRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
