package protocol_test

import (
	"github.com/dobyte/due/v2/internal/stream/internal/codes"
	"github.com/dobyte/due/v2/internal/stream/internal/protocol"
	"github.com/dobyte/due/v2/session"
	"testing"
)

func TestEncodePushReq(t *testing.T) {
	buffer := protocol.EncodePushReq(1, session.User, 3, []byte("hello world"))

	t.Log(buffer.Bytes())
}

func TestDecodePushReq(t *testing.T) {
	buffer := protocol.EncodePushReq(1, session.User, 3, []byte("hello world"))

	seq, kind, target, message, err := protocol.DecodePushReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("kind: %v", kind)
	t.Logf("target: %v", target)
	t.Logf("message: %v", string(message))
}

func TestEncodePushRes(t *testing.T) {
	buffer := protocol.EncodePushRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodePushRes(t *testing.T) {
	buffer := protocol.EncodePushRes(1, codes.OK)

	code, err := protocol.DecodePushRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
