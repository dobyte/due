package packet_test

import (
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"testing"
)

func TestGetIPPacker_Req(t *testing.T) {
	p := packet.NewGetIPPacker()

	buf, err := p.PackReq(1, session.User, 1)
	if err != nil {
		t.Fatal(err)
	}

	seq, kind, target, err := p.UnpackReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v kind: %v target: %v", seq, kind, target)
}

func TestGetIPPacker_Res(t *testing.T) {
	p := packet.NewGetIPPacker()

	buf, err := p.PackRes(1, 0)
	if err != nil {
		t.Fatal(err)
	}

	code, ip, err := p.UnpackRes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v ip: %v", code, ip)

	buf, err = p.PackRes(1, 0, "218.108.212.34")
	if err != nil {
		t.Fatal(err)
	}

	code, ip, err = p.UnpackRes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v ip: %v", code, ip)
}
