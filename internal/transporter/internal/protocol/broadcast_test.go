package protocol_test

import (
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
	"testing"
)

func TestEncodeBroadcastReq(t *testing.T) {
	buffer := protocol.EncodeBroadcastReq(1, session.User, []byte("hello world"))

	t.Log(buffer.Bytes())
}

func TestDecodeBroadcastReq(t *testing.T) {
	buffer := protocol.EncodeBroadcastReq(1, session.User, []byte("hello world"))

	seq, kind, message, err := protocol.DecodeBroadcastReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("kind: %v", kind)
	t.Logf("message: %v", string(message))
}

func TestEncodeBroadcastRes(t *testing.T) {
	buffer := protocol.EncodeBroadcastRes(1, codes.OK, 20)

	t.Log(buffer.Bytes())
}

func TestDecodeBroadcastRes(t *testing.T) {
	buffer := protocol.EncodeBroadcastRes(1, codes.OK, 20)

	code, total, err := protocol.DecodeBroadcastRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("total: %v", total)
}
