package buffer_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

// TestBytes_Methods verifies the Bytes methods and release behaviour.
func TestBytes_Methods(t *testing.T) {
	b := buffer.NewBytes([]byte("abc"))

	if got := b.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	if got := b.Cap(); got != 3 {
		t.Errorf("Cap() = %d, want 3", got)
	}
	if got := b.Available(); got != 0 {
		t.Errorf("Available() = %d, want 0", got)
	}
	if got := b.Nodes(); got != 1 {
		t.Errorf("Nodes() = %d, want 1", got)
	}
	if got, want := string(b.Bytes()), "abc"; got != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}

	var collected string
	if ok := b.VisitBytes(func(data []byte) bool {
		collected += string(data)
		return true
	}); !ok {
		t.Error("VisitBytes() = false, want true")
	}
	if collected != "abc" {
		t.Errorf("VisitBytes() collected %q, want %q", collected, "abc")
	}

	if !b.Slide(1) {
		t.Fatal("Slide(1) = false, want true")
	}
	if got, want := string(b.Bytes()), "bc"; got != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}
	if b.Slide(5) {
		t.Error("Slide(5) = true, want false")
	}
	if b.Slide(-1) {
		t.Error("Slide(-1) = true, want false")
	}

	b.Delay(2)
	b.Release()
	if got, want := string(b.Bytes()), "bc"; got != want {
		t.Errorf("Bytes() after delayed Release = %q, want %q", got, want)
	}

	b.Release()
	if got := b.Len(); got != 0 {
		t.Errorf("Len() after Release = %d, want 0", got)
	}

	b.Release()
	if got := b.Nodes(); got != 1 {
		t.Errorf("Nodes() after extra Release = %d, want 1", got)
	}
}

// TestBytes_WithCapacity verifies a Bytes created from a raw capacity.
func TestBytes_WithCapacity(t *testing.T) {
	b := buffer.NewBytesWithCapacity(4)

	if got := b.Len(); got != 4 {
		t.Errorf("Len() = %d, want 4", got)
	}
	if got := b.Cap(); got != 4 {
		t.Errorf("Cap() = %d, want 4", got)
	}
	if got := len(b.Bytes()); got != 4 {
		t.Errorf("len(Bytes()) = %d, want 4", got)
	}
}

// TestBytes_Static verifies that a static buffer is not reset on release.
func TestBytes_Static(t *testing.T) {
	b := buffer.NewBytes([]byte{1, 2, 3}, true)

	b.Release()

	if got, want := len(b.Bytes()), 3; got != want {
		t.Errorf("len(Bytes()) after Release = %d, want %d", got, want)
	}
}

// TestBytes_Resize verifies that Resize can grow the readable length within the buffer capacity
// while keeping the reserved headroom available for prepending.
func TestBytes_Resize(t *testing.T) {
	b := buffer.MallocBytesWithHeadroom(16, 8)
	if b == nil {
		t.Fatal("MallocBytesWithHeadroom returned nil")
	}
	defer b.Release()

	if got := b.Len(); got != 8 {
		t.Fatalf("Len() = %d, want 8", got)
	}

	if !b.Resize(4) {
		t.Fatal("Resize(4) = false, want true")
	}
	if got := b.Len(); got != 4 {
		t.Errorf("Len() after Resize(4) = %d, want 4", got)
	}

	if !b.Resize(16) {
		t.Fatal("Resize(16) = false, want true")
	}
	if got := b.Len(); got != 16 {
		t.Errorf("Len() after Resize(16) = %d, want 16", got)
	}

	if b.Resize(17) {
		t.Error("Resize beyond capacity = true, want false")
	}
	if b.Resize(-1) {
		t.Error("Resize(-1) = true, want false")
	}

	if header := b.Prepend(4); len(header) != 4 {
		t.Fatalf("Prepend(4) returned %d bytes, want 4", len(header))
	}
	if got, want := b.Len(), 20; got != want {
		t.Errorf("Len() after Prepend = %d, want %d", got, want)
	}
}
