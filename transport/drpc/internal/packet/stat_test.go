package packet_test

import (
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"testing"
)

func TestStatPacker_Req(t *testing.T) {
	p := packet.NewStatPacker()

	buf, err := p.PackReq(1, session.User)
	if err != nil {
		t.Fatal(err)
	}

	seq, kind, err := p.UnpackReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v kind: %v", seq, kind)
}

func TestStatPacker_Res(t *testing.T) {
	p := packet.NewStatPacker()

	buf, err := p.PackRes(1)
	if err != nil {
		t.Fatal(err)
	}

	total, err := p.UnpackRes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("total: %v", total)

	buf, err = p.PackRes(1, 100)
	if err != nil {
		t.Fatal(err)
	}

	total, err = p.UnpackRes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("total: %v", total)
}
