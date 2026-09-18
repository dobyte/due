package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/packet"
)

func TestEncodePublishReq(t *testing.T) {
	message, err := packet.PackMessage(&packet.Message{
		Route:  1,
		Seq:    2,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buf := protocol.EncodePublishReq(1, "channel", true, buffer.NewNocopyBuffer(message))

	t.Log(buf.Bytes())
}

func TestDecodePublishReq(t *testing.T) {
	message, err := packet.PackMessage(&packet.Message{
		Route:  1,
		Seq:    2,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := protocol.EncodePublishReq(1, "channel", true, buffer.NewNocopyBuffer(message))
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	channel, disconnect, data, err := protocol.DecodePublishReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("channel: %v", channel)
	t.Logf("disconnect: %v", disconnect)
	t.Logf("message: %v", string(data.Bytes()))
}

func TestEncodePublishRes(t *testing.T) {
	buf := protocol.EncodePublishRes(1, 0, 1)

	t.Log(buf.Bytes())
}

func TestDecodePublishRes(t *testing.T) {
	req := protocol.EncodePublishRes(1, codes.OK, 20)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, total, err := protocol.DecodePublishRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("total: %v", total)
}
