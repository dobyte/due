package buffer

import "testing"

func TestBytesHeadroom(t *testing.T) {
	buf := MallocBytesWithHeadroom(4, 3)
	if buf == nil {
		t.Fatal("MallocBytesWithHeadroom returned nil")
	}
	defer buf.Release()

	copy(buf.Bytes(), "abc")
	prefix := buf.Prepend(4)
	if prefix == nil {
		t.Fatal("Prepend returned nil")
	}
	copy(prefix, "head")
	if got := string(buf.Bytes()); got != "headabc" {
		t.Fatalf("Bytes() = %q, want %q", got, "headabc")
	}
	if prefix = buf.Prepend(1); prefix != nil {
		t.Fatalf("Prepend beyond headroom = %v, want nil", prefix)
	}
}

func TestBytesHeadroomInvalid(t *testing.T) {
	if buf := MallocBytesWithHeadroom(-1, 1); buf != nil {
		t.Fatal("negative headroom must be rejected")
	}
	if buf := MallocBytesWithHeadroom(1, -1); buf != nil {
		t.Fatal("negative size must be rejected")
	}
	if buf := MallocBytesWithHeadroom(int(^uint(0)>>1), 1); buf != nil {
		t.Fatal("overflowing size must be rejected")
	}
}

func TestBytesPrependRequiresReservedUnsharedHeadroom(t *testing.T) {
	plain := MallocBytes(3)
	if prefix := plain.Prepend(1); prefix != nil {
		t.Fatalf("ordinary buffer Prepend(1) = %v, want nil", prefix)
	}
	plain.Release()

	shared := MallocBytesWithHeadroom(4, 3)
	copy(shared.Bytes(), "abc")
	shared.Delay(2)
	if prefix := shared.Prepend(1); prefix != nil {
		t.Fatalf("shared buffer Prepend(1) = %v, want nil", prefix)
	}
	if got := string(shared.Bytes()); got != "abc" {
		t.Fatalf("shared buffer changed to %q", got)
	}
	shared.Release()
	shared.Release()

	slid := MallocBytesWithHeadroom(4, 3)
	copy(slid.Bytes(), "abc")
	if !slid.Slide(1) {
		t.Fatal("Slide(1) failed")
	}
	if prefix := slid.Prepend(1); prefix != nil {
		t.Fatalf("slid buffer Prepend(1) = %v, want nil", prefix)
	}
	if got := string(slid.Bytes()); got != "bc" {
		t.Fatalf("slid buffer changed to %q", got)
	}
	slid.Release()
}
