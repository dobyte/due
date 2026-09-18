package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
)

func TestEncodePushReq(t *testing.T) {
	message, err := packet.PackMessage(&packet.Message{
		Route:  1,
		Seq:    2,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buf := protocol.EncodePushReq(1, session.User, 3, true, buffer.NewNocopyBuffer(message))

	t.Log(buf.Bytes())
}

func TestDecodePushReq(t *testing.T) {
	message, err := packet.PackMessage(&packet.Message{
		Route:  1,
		Seq:    2,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := protocol.EncodePushReq(1, session.User, 3, true, buffer.NewNocopyBuffer(message))
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, target, disconnect, data, err := protocol.DecodePushReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("target: %v", target)
	t.Logf("disconnect: %v", disconnect)
	t.Logf("message: %v", string(data.Bytes()))
}

func TestEncodePushRes(t *testing.T) {
	buffer := protocol.EncodePushRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodePushRes(t *testing.T) {
	req := protocol.EncodePushRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodePushRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
