package protocol_test

import (
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
	"testing"
)

func TestEncodeGetIPReq(t *testing.T) {
	buffer := protocol.EncodeGetIPReq(1, session.User, 3)

	t.Log(buffer.Bytes())
}

func TestDecodeGetIPReq(t *testing.T) {
	buffer := protocol.EncodeGetIPReq(1, session.User, 3)

	seq, kind, target, err := protocol.DecodeGetIPReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("kind: %v", kind)
	t.Logf("target: %v", target)
}

func TestEncodeGetIPRes(t *testing.T) {
	buffer := protocol.EncodeGetIPRes(1, codes.OK, "127.0.0.1")

	t.Log(buffer.Bytes())
}

func TestDecodeGetIPRes(t *testing.T) {
	buffer := protocol.EncodeGetIPRes(1, codes.OK, "127.0.0.1")

	code, ip, err := protocol.DecodeGetIPRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
	t.Logf("ip: %v", ip)
}
