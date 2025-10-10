package protocol_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeDeliverReq(t *testing.T) {
	buffer := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewNocopyBuffer([]byte("hello world")))

	t.Log(buffer.Bytes())
}

func TestDecodeDeliverReq(t *testing.T) {
	buffer := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewNocopyBuffer([]byte("hello world")))

	seq, cid, uid, message, err := protocol.DecodeDeliverReq(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v", seq)
	t.Logf("cid: %v", cid)
	t.Logf("uid: %v", uid)
	t.Logf("message: %v", string(message))
}

func TestEncodeDeliverRes(t *testing.T) {
	buffer := protocol.EncodeDeliverRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeDeliverRes(t *testing.T) {
	buffer := protocol.EncodePushRes(1, codes.OK)

	code, err := protocol.DecodeDeliverRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
