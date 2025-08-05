package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
)

func TestEncodeSubscribeReq(t *testing.T) {
	buf := protocol.EncodeSubscribeReq(1, session.User, []int64{1, 2, 3}, "channel")

	t.Log(buf.Bytes())
}

func TestDecodeSubscribeReq(t *testing.T) {
	buf := protocol.EncodeSubscribeReq(1, session.User, []int64{1, 2, 3}, "channel")

	seq, kind, targets, channel, err := protocol.DecodeSubscribeReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("kind: %v", kind)
	t.Logf("targets: %v", targets)
	t.Logf("channel: %v", channel)
}

func TestEncodeSubscribeRes(t *testing.T) {
	buffer := protocol.EncodeSubscribeRes(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeSubscribeRes(t *testing.T) {
	buffer := protocol.EncodeSubscribeRes(1, 2)

	code, err := protocol.DecodeSubscribeRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
