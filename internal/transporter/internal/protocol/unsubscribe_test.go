package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeUnsubscribeReq(t *testing.T) {
	buf := protocol.EncodeUnsubscribeReq(1, cluster.User, []int64{1, 2, 3}, "channel")

	t.Log(buf.Bytes())
}

func TestDecodeUnsubscribeReq(t *testing.T) {
	req := protocol.EncodeUnsubscribeReq(1, cluster.User, []int64{1, 2, 3}, "channel")
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, targets, channel, err := protocol.DecodeUnsubscribeReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("targets: %v", targets)
	t.Logf("channel: %v", channel)
}

func TestEncodeUnsubscribeRes(t *testing.T) {
	buffer := protocol.EncodeUnsubscribeRes(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeUnsubscribeRes(t *testing.T) {
	req := protocol.EncodeUnsubscribeRes(1, 2)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeUnsubscribeRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
