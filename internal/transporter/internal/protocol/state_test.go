package protocol_test

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"testing"
)

func TestDecodeGetStateReq(t *testing.T) {
	buf := protocol.EncodeGetStateReq(1)

	seq, err := protocol.DecodeGetStateReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
}

func TestDecodeGetStateRes(t *testing.T) {
	buf := protocol.EncodeGetStateRes(1, codes.OK, cluster.Work)

	code, state, err := protocol.DecodeGetStateRes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("state: %v", state)
}

func TestDecodeSetStateReq(t *testing.T) {
	buf := protocol.EncodeSetStateReq(1, cluster.Shut)

	seq, state, err := protocol.DecodeSetStateReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("state: %v", state)
}

func TestDecodeSetStateRes(t *testing.T) {
	buf := protocol.EncodeSetStateRes(1, codes.OK)

	code, err := protocol.DecodeSetStateRes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
