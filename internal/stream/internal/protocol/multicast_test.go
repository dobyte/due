package protocol_test

import (
	"github.com/dobyte/due/v2/internal/stream/internal/codes"
	"github.com/dobyte/due/v2/internal/stream/internal/protocol"
	"github.com/dobyte/due/v2/session"
	"testing"
)

func TestEncodeMulticastReq(t *testing.T) {
	buffer := protocol.EncodeMulticastReq(1, session.User, []int64{1, 2, 3}, []byte("hello world"))

	t.Log(buffer.Bytes())
}

func TestDecodeMulticastReq(t *testing.T) {
	buffer := protocol.EncodeMulticastReq(1, session.User, []int64{1, 2, 3}, []byte("hello world"))

	seq, kind, targets, message, err := protocol.DecodeMulticastReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("kind: %v", kind)
	t.Logf("targets: %v", targets)
	t.Logf("message: %v", string(message))
}

func TestEncodeMulticastRes(t *testing.T) {
	buffer := protocol.EncodeMulticastRes(1, codes.OK, 20)

	t.Log(buffer.Bytes())
}

func TestDecodeMulticastRes(t *testing.T) {
	buffer := protocol.EncodeMulticastRes(1, codes.OK, 20)

	code, total, err := protocol.DecodeMulticastRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("total: %v", total)
}
