package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
)

func TestEncodeUnsubscribeReq(t *testing.T) {
	buf := protocol.EncodeUnsubscribeReq(1, session.User, []int64{1, 2, 3}, "channel")

	t.Log(buf.Bytes())
}

func TestDecodeUnsubscribeReq(t *testing.T) {
	buf := protocol.EncodeUnsubscribeReq(1, session.User, []int64{1, 2, 3}, "channel")

	seq, kind, targets, channel, err := protocol.DecodeUnsubscribeReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("kind: %v", kind)
	t.Logf("targets: %v", targets)
	t.Logf("channel: %v", channel)
}

func TestEncodeUnsubscribeRes(t *testing.T) {
	buffer := protocol.EncodeUnsubscribeRes(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeUnsubscribeRes(t *testing.T) {
	buffer := protocol.EncodeUnsubscribeRes(1, 2)

	code, err := protocol.DecodeUnsubscribeRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
