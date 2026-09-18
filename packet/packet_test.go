package packet_test

import (
	"bytes"
	"testing"

	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xrand"
)

var packer = packet.NewPacker(
	packet.WithHeartbeatTime(false),
)

func TestDefaultPacker_Read(t *testing.T) {
	buf1, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer buf1.Release()

	isHeartbeat, heartbeatTime, buf2, err := packer.Read(bytes.NewReader(buf1.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	defer buf2.Release()

	route, seq, buf, err := packer.UnpackMessage(buf2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("isHeartbeat: %v", isHeartbeat)
	t.Logf("heartbeatTime: %d", heartbeatTime)
	t.Logf("route: %d", route)
	t.Logf("seq: %d", seq)
	t.Logf("buffer: %s", string(buf.Bytes()))
}

func TestDefaultPacker_UnpackMessage(t *testing.T) {
	data, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(data)

	route, seq, buf, err := packer.UnpackMessage(data)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("route: %d", route)
	t.Logf("seq: %d", seq)
	t.Logf("buffer: %s", string(buf.Bytes()))
}

func TestDefaultPacker_PackHeartbeat(t *testing.T) {
	buf := packer.PackHeartbeat()
	t.Log(buf.Bytes())
}

func BenchmarkDefaultPacker_Read(b *testing.B) {
	msg, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte(xrand.Letters(2048)),
	})
	if err != nil {
		b.Fatal(err)
	}

	reader := bytes.NewReader(msg.Bytes())

	defer msg.Release()

	b.ResetTimer()
	b.SetBytes(int64(msg.Len()))

	for b.Loop() {
		if _, _, buf, err := packer.Read(reader); err != nil {
			b.Fatal(err)
		} else if buf != nil {
			buf.Release()
		}

		reader.Reset(msg.Bytes())
	}
}

func BenchmarkDefaultPacker_PackMessage(b *testing.B) {
	buffer := []byte(xrand.Letters(1024))

	b.ResetTimer()
	b.SetBytes(int64(len(buffer)))

	for b.Loop() {
		_, err := packer.PackMessage(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: buffer,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDefaultPacker_UnpackMessage(b *testing.B) {
	buf, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte(xrand.Letters(1024)),
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.SetBytes(int64(buf.Len()))

	for b.Loop() {
		if _, _, _, err := packer.UnpackMessage(buf); err != nil {
			b.Fatal(err)
		}
	}
}
