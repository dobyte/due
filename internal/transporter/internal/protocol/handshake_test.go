package protocol_test

import (
	"fmt"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/dobyte/due/v2/utils/xuuid"
)

func TestEncodeHandshakeReq(t *testing.T) {
	buffer := protocol.EncodeHandshakeReq(1, cluster.Gate, xuuid.UUID(), uint64(xtime.Now().UnixNano()))

	t.Log(buffer.Bytes())
}

func TestDecodeHandshakeReq(t *testing.T) {
	buf1 := protocol.EncodeHandshakeReq(1, cluster.Gate, xuuid.UUID(), uint64(xtime.Now().UnixNano()))
	buf2 := buffer.NewBytes(buf1.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])

	fmt.Println(buf1.Bytes())
	fmt.Println(buf2.Bytes())

	kind, iid, epoch, err := protocol.DecodeHandshakeReq(buf2)
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
	buffer := protocol.EncodeHandshakeRes(1, codes.OK)

	code, err := protocol.DecodeHandshakeRes(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
