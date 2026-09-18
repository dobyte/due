package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeTriggerReq(t *testing.T) {
	buffer := protocol.EncodeTriggerReq(1, cluster.Disconnect, 1)

	t.Log(buffer.Bytes())
}

func TestDecodeTriggerReq(t *testing.T) {
	req := protocol.EncodeTriggerReq(1, cluster.Disconnect, 1, 2)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	evt, cid, uid, err := protocol.DecodeTriggerReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("evt: %v", evt)
	t.Logf("cid: %v", cid)
	t.Logf("uid: %v", uid)
}

func TestEncodeTriggerRes(t *testing.T) {
	buffer := protocol.EncodeTriggerRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeTriggerRes(t *testing.T) {
	req := protocol.EncodeTriggerRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeTriggerRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
