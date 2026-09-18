package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeUnbindReq(t *testing.T) {
	buffer := protocol.EncodeUnbindReq(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeUnbindReq(t *testing.T) {
	req := protocol.EncodeUnbindReq(1, 2)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	uid, err := protocol.DecodeUnbindReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("uid: %v", uid)
}

func TestEncodeUnbindRes(t *testing.T) {
	buffer := protocol.EncodeUnbindRes(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeUnbindRes(t *testing.T) {
	req := protocol.EncodeUnbindRes(1, 2)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeUnbindRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
