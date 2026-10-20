package packet_test

import (
	"testing"

	"github.com/dobyte/due/v2/packet"
)

// TestPackHandshake verifies that a handshake frame is a standalone packet kind: it is marked by
// the header bit and carries neither a route nor a sequence number.
func TestPackHandshake(t *testing.T) {
	frame := []byte("frame")

	buf, err := packet.PackHandshake(frame)
	if err != nil {
		t.Fatalf("pack handshake frame failed: %v", err)
	}

	if len(buf.Bytes()) != 4+1+len(frame) {
		t.Fatalf("unexpected handshake frame size: %d, want %d", len(buf.Bytes()), 4+1+len(frame))
	}

	payload, err := packet.ExtractHandshake(buf)
	if err != nil {
		t.Fatalf("extract handshake frame failed: %v", err)
	}

	if string(payload) != string(frame) {
		t.Fatalf("unexpected handshake payload: %s", payload)
	}

	if _, _, _, err = packet.ExtractMessage(buf); err == nil {
		t.Fatal("a handshake frame must not be extracted as a data message")
	}

	if _, _, err = packet.ExtractRouteSeq(buf); err == nil {
		t.Fatal("a handshake frame must not expose a route or seq")
	}

	if _, _, _, err = packet.UnpackMessage(buf); err == nil {
		t.Fatal("a handshake frame must not be unpacked as a data message")
	}

	msg, err := packet.PackMessage(100, 1, []byte("data"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	if _, err = packet.ExtractHandshake(msg); err == nil {
		t.Fatal("a data message must not be extracted as a handshake frame")
	}

	route, seq, data, err := packet.ExtractMessage(msg)
	if err != nil {
		t.Fatalf("extract message failed: %v", err)
	}

	if route != 100 || seq != 1 || string(data) != "data" {
		t.Fatalf("unexpected data message: route=%d seq=%d payload=%s", route, seq, data)
	}
}
