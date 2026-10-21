package packet_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// legacyBufferPacker intentionally implements only the original Packer interface.
type legacyBufferPacker struct {
	packet.Packer
}

// TestPooledMessageOwnership covers native and custom packers, including error ownership.
func TestPooledMessageOwnership(t *testing.T) {
	original := packet.GetPacker()
	t.Cleanup(func() { packet.SetPacker(original) })
	for _, packer := range []packet.Packer{packet.NewPacker(), &legacyBufferPacker{Packer: packet.NewPacker()}} {
		packet.SetPacker(packer)
		payload := buffer.MallocBytes(5)
		copy(payload.Bytes(), "hello")
		packed, err := packet.PackMessageBuffer(10, 20, payload)
		if err != nil {
			t.Fatal(err)
		}
		route, seq, data, err := packet.ExtractMessage(packed)
		if err != nil || route != 10 || seq != 20 || string(data) != "hello" {
			t.Fatalf("pooled message: route=%d seq=%d data=%q err=%v", route, seq, data, err)
		}
		// Delay release models a queued buffer shared by multiple consumers.
		packed.Delay(2)
		packed.Release()
		_, _, data, err = packet.ExtractMessage(packed)
		if err != nil || string(data) != "hello" {
			t.Fatal("payload released before the final consumer")
		}
		packed.Release()
		if payload.Len() != 0 {
			t.Fatal("payload ownership was not released")
		}

		payload = buffer.MallocBytes(5)
		copy(payload.Bytes(), "hello")
		if _, err = packet.PackMessageBuffer(1<<30, 1, payload); !errors.Is(err, errors.ErrRouteOverflow) {
			t.Fatalf("unexpected packing error: %v", err)
		}
		if string(payload.Bytes()) != "hello" {
			t.Fatal("packing failure consumed caller's payload")
		}
		payload.Release()
	}
}

func TestPooledMessageHeadroom(t *testing.T) {
	original := packet.GetPacker()
	t.Cleanup(func() { packet.SetPacker(original) })

	for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		for _, routeSize := range []int{1, 2, 4} {
			for _, seqSize := range []int{0, 1, 2, 4} {
				packer := packet.NewPacker(
					packet.WithByteOrder(order),
					packet.WithRouteBytes(routeSize),
					packet.WithSeqBytes(seqSize),
				)
				packet.SetPacker(packer)
				headroom := packet.MessageBufferHeadroom()
				payload := buffer.MallocBytesWithHeadroom(headroom, len("payload"))
				copy(payload.Bytes(), "payload")
				packed, err := packet.PackMessageBuffer(-10, -20, payload)
				if err != nil {
					t.Fatal(err)
				}
				if packed != payload || packed.Nodes() != 1 {
					t.Fatal("headroom payload was wrapped instead of prepended in place")
				}
				route, seq, data, err := packet.ExtractMessage(packed)
				wantSeq := int32(-20)
				if seqSize == 0 {
					wantSeq = 0
				}
				if err != nil || route != -10 || seq != wantSeq || string(data) != "payload" {
					t.Fatalf("route=%d seq=%d data=%q err=%v", route, seq, data, err)
				}
				packed.Release()
			}
		}
	}
}

func TestPooledMessageSharedHeadroom(t *testing.T) {
	packer := packet.NewPacker(packet.WithRouteBytes(1), packet.WithSeqBytes(0))
	payload := buffer.MallocBytesWithHeadroom(packer.MessageBufferHeadroom(), len("payload"))
	copy(payload.Bytes(), "payload")
	payload.Delay(2)

	first, err := packer.PackMessageBuffer(1, 0, payload)
	if err != nil {
		t.Fatal(err)
	}
	if first.Nodes() != 2 {
		t.Fatalf("first shared message has %d nodes, want 2", first.Nodes())
	}
	firstBytes := bytes.Clone(first.Bytes())

	second, err := packer.PackMessageBuffer(2, 0, payload)
	if err != nil {
		t.Fatal(err)
	}
	if second.Nodes() != 2 {
		t.Fatalf("second shared message has %d nodes, want 2", second.Nodes())
	}
	if !bytes.Equal(first.Bytes(), firstBytes) {
		t.Fatal("second pack changed the first shared message")
	}
	for _, tc := range []struct {
		name   string
		packed buffer.Buffer
		route  int32
	}{
		{name: "first", packed: first, route: 1},
		{name: "second", packed: second, route: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route, seq, data, err := packer.ExtractMessage(tc.packed)
			if err != nil || route != tc.route || seq != 0 || string(data) != "payload" {
				t.Fatalf("route=%d seq=%d data=%q err=%v", route, seq, data, err)
			}
		})
	}
	first.Release()
	if got := string(payload.Bytes()); got != "payload" {
		t.Fatalf("payload after first release = %q", got)
	}
	second.Release()
	if payload.Len() != 0 {
		t.Fatal("shared payload was not released after both messages")
	}
}

func TestPooledMessageSlidHeadroom(t *testing.T) {
	packer := packet.NewPacker()
	payload := buffer.MallocBytesWithHeadroom(packer.MessageBufferHeadroom(), len("payload"))
	copy(payload.Bytes(), "payload")
	if !payload.Slide(1) {
		t.Fatal("Slide(1) failed")
	}
	packed, err := packer.PackMessageBuffer(1, 2, payload)
	if err != nil {
		t.Fatal(err)
	}
	if packed.Nodes() != 2 {
		t.Fatalf("slid payload has %d nodes, want 2", packed.Nodes())
	}
	route, seq, data, err := packer.ExtractMessage(packed)
	if err != nil || route != 1 || seq != 2 || string(data) != "ayload" {
		t.Fatalf("route=%d seq=%d data=%q err=%v", route, seq, data, err)
	}
	packed.Release()
}

// TestExtractSegmentedMessage covers every split position, nesting and packet configuration.
func TestExtractSegmentedMessage(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		for _, routeSize := range []int{1, 2, 4} {
			for _, seqSize := range []int{0, 1, 2, 4} {
				p := packet.NewPacker(packet.WithByteOrder(order), packet.WithRouteBytes(routeSize), packet.WithSeqBytes(seqSize))
				for _, payload := range [][]byte{nil, []byte("payload")} {
					packed, err := p.PackMessage(-10, -20, payload)
					if err != nil {
						t.Fatal(err)
					}
					raw := bytes.Clone(packed.Bytes())
					packed.Release()
					for split := 0; split <= len(raw); split++ {
						buf := buffer.NewNocopyBuffer(buffer.NewNocopyBuffer(raw[:split]), raw[split:])
						route, seq, data, err := p.ExtractMessage(buf)
						wantSeq := int32(-20)
						if seqSize == 0 {
							wantSeq = 0
						}
						if err != nil || route != -10 || seq != wantSeq || !bytes.Equal(data, payload) {
							t.Fatalf("split %d: route=%d seq=%d data=%q err=%v", split, route, seq, data, err)
						}
						var after []byte
						buf.VisitBytes(func(block []byte) bool {
							after = append(after, block...)
							return true
						})
						if !bytes.Equal(after, raw) {
							t.Fatal("extract modified a shared message")
						}
						buf.Release()
					}
				}
			}
		}
	}
}

func TestExtractSegmentedMessageAliasesPayload(t *testing.T) {
	p := packet.NewPacker()
	payload := []byte("payload")
	packed, err := p.PackMessage(10, 1, payload)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Release()
	_, _, data, err := p.ExtractMessage(packed)
	if err != nil || &data[0] != &payload[0] {
		t.Fatal("a contiguous payload was copied")
	}
}
