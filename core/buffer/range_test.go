package buffer

import (
	"bytes"
	"testing"
)

// TestNocopySlice covers nested nodes, empty nodes and all valid ranges without consuming data.
func TestNocopySlice(t *testing.T) {
	data := []byte("abcdefgh")
	buf := NewNocopyBuffer(NewBytes(nil, true), data[:2],
		NewNocopyBuffer(data[2:5], NewNocopyBuffer(data[5:7])), data[7:])
	defer buf.Release()
	for lower := 0; lower <= len(data); lower++ {
		for upper := lower; upper <= len(data); upper++ {
			if got := buf.Slice(lower, upper); !bytes.Equal(got, data[lower:upper]) {
				t.Fatalf("range [%d,%d): got %q", lower, upper, got)
			}
		}
	}
	if got := buf.Slice(3, 5); &got[0] != &data[3] {
		t.Fatal("a contiguous range must alias its block")
	}
	for _, bounds := range [][2]int{{-1, 2}, {3, 2}, {0, 9}} {
		if buf.Slice(bounds[0], bounds[1]) != nil {
			t.Fatalf("accepted invalid bounds: %v", bounds)
		}
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("slice modified the buffer")
	}
}

func TestBytesTruncate(t *testing.T) {
	buf := NewBytes([]byte("prefix-body-tail"), true)
	if !buf.Slide(7) || !buf.Truncate(4) || string(buf.Bytes()) != "body" {
		t.Fatalf("unexpected truncated view: %q", buf.Bytes())
	}
	if buf.Truncate(-1) || buf.Truncate(5) || string(buf.Bytes()) != "body" {
		t.Fatal("invalid truncation modified the buffer")
	}
	if !buf.Truncate(0) || buf.Len() != 0 {
		t.Fatal("truncate to zero failed")
	}
}
