package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
)

func TestEncodeSubscribeReq(t *testing.T) {
	buf := protocol.EncodeSubscribeReq(1, session.User, []int64{1, 2, 3}, "channel")

	t.Log(buf.Bytes())
}

func TestDecodeSubscribeReq(t *testing.T) {
	req := protocol.EncodeSubscribeReq(1, session.User, []int64{1, 2, 3}, "channel")
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, targets, channel, err := protocol.DecodeSubscribeReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("targets: %v", targets)
	t.Logf("channel: %v", channel)
}

func TestEncodeSubscribeRes(t *testing.T) {
	buffer := protocol.EncodeSubscribeRes(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeSubscribeRes(t *testing.T) {
	req := protocol.EncodeSubscribeRes(1, 2)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeSubscribeRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
