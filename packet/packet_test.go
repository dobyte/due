package packet_test

import (
	"encoding/binary"
	"github.com/dobyte/due/packet"
	"testing"
)

func TestPacket(t *testing.T) {
	packet.SetPacker(packet.NewPacker(
		packet.WithByteOrder(binary.BigEndian),
		packet.WithSeqBytesLen(0),
	))

	data, err := packet.Pack(&packet.Message{
		Seq:    -65536,
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
