package protocol_test

import (
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"testing"
)

func TestEncodeBindReq(t *testing.T) {
	buffer := protocol.EncodeBindReq(1, 2, 3)

	t.Log(buffer.Bytes())
}

func TestDecodeBindReq(t *testing.T) {
	buffer := protocol.EncodeBindReq(1, 2, 3)

	seq, cid, uid, err := protocol.DecodeBindReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("cid: %v", cid)
	t.Logf("uid: %v", uid)
}

func TestEncodeBindRes(t *testing.T) {
	buffer := protocol.EncodeBindRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeBindRes(t *testing.T) {
	buffer := protocol.EncodeBindRes(1, codes.OK)

	code, err := protocol.DecodeBindRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
