package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeDeliverReq(t *testing.T) {
	buffer := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewNocopyBuffer([]byte("hello world")))

	t.Log(buffer.Bytes())
}

func TestDecodeDeliverReq(t *testing.T) {
	req := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewNocopyBuffer([]byte("hello world")))
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	cid, uid, data, err := protocol.DecodeDeliverReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("cid: %v", cid)
	t.Logf("uid: %v", uid)
	t.Logf("message: %v", string(data.Bytes()))
}

func TestEncodeDeliverRes(t *testing.T) {
	buffer := protocol.EncodeDeliverRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeDeliverRes(t *testing.T) {
	req := protocol.EncodeDeliverRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeDeliverRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
