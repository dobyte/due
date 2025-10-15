package packet_test

import (
	"bytes"
	"testing"

	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xrand"
)

var packer = packet.NewPacker(
	packet.WithHeartbeatTime(true),
)

func TestDefaultPacker_ReadMessage(t *testing.T) {
	data, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(data)

	reader := bytes.NewReader(data)

	message, err := packer.ReadMessage(reader)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(message)
}

func TestDefaultPacker_PackBuffer(t *testing.T) {
	buf, err := packer.PackBuffer(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(buf.Bytes())

	message, err := packer.UnpackMessage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	buf.Release()

	t.Logf("seq: %d", message.Seq)
	t.Logf("route: %d", message.Route)
	t.Logf("buffer: %s", string(message.Buffer))
}

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

func TestDefaultPacker_PackHeartbeat(t *testing.T) {
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

func BenchmarkDefaultPacker_ReadBuffer(b *testing.B) {
	data, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte(xrand.Letters(2048)),
	})
	if err != nil {
		b.Fatal(err)
	}

	reader := bytes.NewReader(data)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		if buf, err := packer.ReadBuffer(reader); err != nil {
			b.Fatal(err)
		} else {
			buf.Release()
		}

		reader.Reset(data)
	}
}

func BenchmarkDefaultPacker_ReadMessage(b *testing.B) {
	data, err := packer.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte(xrand.Letters(2048)),
	})
	if err != nil {
		b.Fatal(err)
	}

	reader := bytes.NewReader(data)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		if _, err = packer.ReadMessage(reader); err != nil {
			b.Fatal(err)
		}

		reader.Reset(data)
	}
}

func BenchmarkDefaultPacker_PackBuffer(b *testing.B) {
	buffer := []byte(xrand.Letters(1024))

	b.ResetTimer()
	b.SetBytes(int64(len(buffer)))

	for i := 0; i < b.N; i++ {
		buf, err := packer.PackBuffer(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: buffer,
		})
		if err != nil {
			b.Fatal(err)
		}

		buf.Release()
	}
}

func BenchmarkDefaultPacker_PackMessage(b *testing.B) {
	buffer := []byte(xrand.Letters(1024))

	b.ResetTimer()
	b.SetBytes(int64(len(buffer)))

	for i := 0; i < b.N; i++ {
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
	b.SetBytes(int64(len(buf)))

	for i := 0; i < b.N; i++ {
		if _, err := packer.UnpackMessage(buf); err != nil {
			b.Fatal(err)
		}
	}
}
