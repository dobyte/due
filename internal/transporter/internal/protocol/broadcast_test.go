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

func TestEncodeBroadcastReq(t *testing.T) {
	message, err := packet.PackMessage(&packet.Message{
		Route:  1,
		Seq:    2,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buf := protocol.EncodeBroadcastReq(1, session.User, true, buffer.NewNocopyBuffer(message))

	t.Log(buf.Bytes())
}

func TestDecodeBroadcastReq(t *testing.T) {
	message, err := packet.PackMessage(&packet.Message{
		Route:  1,
		Seq:    2,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := protocol.EncodeBroadcastReq(1, session.User, true, buffer.NewNocopyBuffer(message))
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, disconnect, data, err := protocol.DecodeBroadcastReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("disconnect: %v", disconnect)
	t.Logf("message: %v", string(data.Bytes()))
}

func TestEncodeBroadcastRes(t *testing.T) {
	buf := protocol.EncodeBroadcastRes(1, codes.OK, 20)

	t.Log(buf.Bytes())
}

func TestDecodeBroadcastRes(t *testing.T) {
	req := protocol.EncodeBroadcastRes(1, codes.OK, 20)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, total, err := protocol.DecodeBroadcastRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("total: %v", total)
}
