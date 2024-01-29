package packet_test

import (
	"github.com/dobyte/due/v2/packet"
	"testing"
)

var packer = packet.NewPacker(
	packet.WithHeartbeatTime(true),
)

func TestDefaultPacker_PackMessage(t *testing.T) {
	data, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(data)

	message, err := packer.UnpackMessage(data)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %d", message.Seq)
	t.Logf("route: %d", message.Route)
	t.Logf("buffer: %s", string(message.Buffer))
}

func TestPackHeartbeat(t *testing.T) {
	data, err := packer.PackHeartbeat()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(data)

	isHeartbeat, err := packer.CheckHeartbeat(data)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(isHeartbeat)
}

func BenchmarkPack(b *testing.B) {
	buffer := []byte("hello world")

	for i := 0; i < b.N; i++ {
		_, err := packet.PackMessage(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: buffer,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnpack(b *testing.B) {
	buf, err := packet.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		_, err := packet.UnpackMessage(buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}
