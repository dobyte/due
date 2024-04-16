package packet_test

import (
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"testing"
)

func TestBindPacker_Req(t *testing.T) {
	p := packet.NewBindPacker()

	buf, err := p.PackReq(1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}

	seq, cid, uid, err := p.UnpackReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v cid: %v uid: %v", seq, cid, uid)
}

func TestBindPacker_Res(t *testing.T) {
	p := packet.NewBindPacker()

	buf, err := p.PackRes(1, 2)
	if err != nil {
		t.Fatal(err)
	}

	code, err := p.UnpackRes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
