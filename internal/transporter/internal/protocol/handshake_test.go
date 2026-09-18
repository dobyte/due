package protocol_test

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/utils/xuuid"
)

func TestEncodeHandshakeReq(t *testing.T) {
	buffer := protocol.EncodeHandshakeReq(1, cluster.Gate, xuuid.UUID(), uint64(time.Now().UnixNano()))

	t.Log(buffer.Bytes())
}

func TestDecodeHandshakeReq(t *testing.T) {
	req := protocol.EncodeHandshakeReq(1, cluster.Gate, xuuid.UUID(), uint64(time.Now().UnixNano()))
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, iid, epoch, err := protocol.DecodeHandshakeReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("inst: %v", iid)
	t.Logf("epoch: %v", epoch)
}

func TestEncodeHandshakeRes(t *testing.T) {
	buffer := protocol.EncodeHandshakeRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeHandshakeRes(t *testing.T) {
	req := protocol.EncodeHandshakeRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeHandshakeRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
