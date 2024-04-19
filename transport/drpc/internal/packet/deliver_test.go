package packet_test

import (
	packets "github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"github.com/dobyte/due/v2/utils/xuuid"
	"testing"
)

func TestDeliverPacker_Req(t *testing.T) {
	p := packet.NewDeliverPacker()

	gidr := xuuid.UUID()

	buf, err := p.PackReq2(1, gidr, 10, 22, &packets.Message{
		Route:  30,
		Seq:    20,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	seq, gid, cid, uid, message, err := p.UnpackReq(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %v gid: %v cid: %v uid: %v message seq: %v message route: %v message buffer: %v", seq, gid, cid, uid, message.Route, message.Seq, message.Buffer)
}

func BenchmarkDeliverPacker_PackReq(b *testing.B) {
	p := packet.NewDeliverPacker()
	gidr := xuuid.UUID()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf, _ := p.PackReq(1, gidr, 10, 22, &packets.Message{
			Route:  30,
			Seq:    20,
			Buffer: []byte("hello world"),
		})
		buf.Recycle()
	}
}

func BenchmarkDeliverPacker_PackReq2(b *testing.B) {
	p := packet.NewDeliverPacker()
	gidr := xuuid.UUID()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf, _ := p.PackReq2(1, gidr, 10, 22, &packets.Message{
			Route:  30,
			Seq:    20,
			Buffer: []byte("hello world"),
		})
		buf.Recycle()
	}
}
