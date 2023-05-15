package packet_test

import (
	"github.com/dobyte/due/packet"
	"testing"
)

func TestPacket(t *testing.T) {
	data, err := packet.Pack(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	message, err := packet.Unpack(data)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("seq: %d", message.Seq)
	t.Logf("route: %d", message.Route)
	t.Logf("buffer: %s", string(message.Buffer))
}

func BenchmarkPack(b *testing.B) {
	buffer := []byte("hello world")

	for i := 0; i < b.N; i++ {
		_, err := packet.Pack(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: buffer,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPack1(b *testing.B) {
	buffer := []byte("hello world")

	for i := 0; i < b.N; i++ {
		_, err := packet.Pack1(&packet.Message{
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
	buf, err := packet.Pack(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		_, err := packet.Unpack(buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnpack1(b *testing.B) {
	buf, err := packet.Pack(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		_, err := packet.Unpack1(buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}
