package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeBindReq(t *testing.T) {
	buffer := protocol.EncodeBindReq(1, 2, 3)

	t.Log(buffer.Bytes())
}

func TestDecodeBindReq(t *testing.T) {
	req := protocol.EncodeBindReq(1, 2, 3)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	cid, uid, err := protocol.DecodeBindReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("cid: %v", cid)
	t.Logf("uid: %v", uid)
}

func TestEncodeBindRes(t *testing.T) {
	buffer := protocol.EncodeBindRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeBindRes(t *testing.T) {
	req := protocol.EncodeBindRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeBindRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
