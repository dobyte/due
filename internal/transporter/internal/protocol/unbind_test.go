package protocol_test

import (
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"testing"
)

func TestEncodeUnbindReq(t *testing.T) {
	buffer := protocol.EncodeUnbindReq(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeUnbindReq(t *testing.T) {
	buffer := protocol.EncodeUnbindReq(1, 2)

	seq, uid, err := protocol.DecodeUnbindReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("uid: %v", uid)
}

func TestEncodeUnbindRes(t *testing.T) {
	buffer := protocol.EncodeUnbindRes(1, 2)

	t.Log(buffer.Bytes())
}

func TestDecodeUnbindRes(t *testing.T) {
	buffer := protocol.EncodeUnbindRes(1, 2)

	code, err := protocol.DecodeUnbindRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
