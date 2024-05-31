package protocol_test

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"testing"
)

func TestEncodeTriggerReq(t *testing.T) {
	buffer := protocol.EncodeTriggerReq(1, cluster.Disconnect, 1)

	t.Log(buffer.Bytes())
}

func TestDecodeTriggerReq(t *testing.T) {
	buffer := protocol.EncodeTriggerReq(1, cluster.Disconnect, 1, 2)

	seq, evt, cid, uid, err := protocol.DecodeTriggerReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("evt: %v", evt)
	t.Logf("cid: %v", cid)
	t.Logf("uid: %v", uid)
}

func TestEncodeTriggerRes(t *testing.T) {
	buffer := protocol.EncodeTriggerRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeTriggerRes(t *testing.T) {
	buffer := protocol.EncodeTriggerRes(1, codes.OK)

	code, err := protocol.DecodeTriggerRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
