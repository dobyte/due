package buffer_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

// TestWriter_Methods verifies the basic Writer methods and release behaviour.
func TestWriter_Methods(t *testing.T) {
	w := buffer.NewWriterWithCapacity()

	if got := w.Cap(); got != 0 {
		t.Errorf("Cap() = %d, want 0", got)
	}
	if got := w.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
	if got := w.Nodes(); got != 1 {
		t.Errorf("Nodes() = %d, want 1", got)
	}

	n, err := w.Write([]byte("ab"))
	if err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("Write() = %d, want 2", n)
	}
	if got := w.Cap(); got != 2 {
		t.Errorf("Cap() after Write = %d, want 2", got)
	}

	w.Grow(10)
	if got := w.Cap(); got != 12 {
		t.Errorf("Cap() after Grow = %d, want 12", got)
	}
	if got := w.Available(); got != 10 {
		t.Errorf("Available() = %d, want 10", got)
	}

	if got, want := string(w.Bytes()), "ab"; got != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}

	if !w.Slide(1) {
		t.Fatal("Slide(1) = false, want true")
	}
	if got, want := string(w.Bytes()), "b"; got != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}
	if w.Slide(5) {
		t.Error("Slide(5) = true, want false")
	}
	if w.Slide(-1) {
		t.Error("Slide(-1) = true, want false")
	}

	var collected string
	if ok := w.VisitBytes(func(data []byte) bool {
		collected += string(data)
		return true
	}); !ok {
		t.Error("VisitBytes() = false, want true")
	}
	if collected != "b" {
		t.Errorf("VisitBytes() collected %q, want %q", collected, "b")
	}

	w.Delay(2)
	w.Release()
	if got, want := string(w.Bytes()), "b"; got != want {
		t.Errorf("Bytes() after delayed Release = %q, want %q", got, want)
	}

	w.Release()
	if got := w.Len(); got != 0 {
		t.Errorf("Len() after Release = %d, want 0", got)
	}

	w.Release()
	if got := w.Nodes(); got != 1 {
		t.Errorf("Nodes() after extra Release = %d, want 1", got)
	}
}

// TestWriter_Static verifies that a static writer is not reset on release.
func TestWriter_Static(t *testing.T) {
	w := buffer.NewWriter([]byte{1, 2, 3}, true)
	w.WriteBytes(4)

	w.Release()

	if got, want := len(w.Bytes()), 1; got != want {
		t.Errorf("len(Bytes()) after Release = %d, want %d", got, want)
	}

	nonStatic := buffer.NewWriter([]byte{1, 2, 3})
	nonStatic.WriteBytes(4)
	nonStatic.Release()

	if got := nonStatic.Len(); got != 0 {
		t.Errorf("Len() after Release = %d, want 0", got)
	}
}

// TestWriterPool_Bounds verifies the writer pool bounds.
func TestWriterPool_Bounds(t *testing.T) {
	pool := buffer.NewWriterPoolWithCapacity(1024)

	if w := pool.Get(20); w == nil {
		t.Error("Get(20) = nil, want non-nil")
	} else if got, want := w.Cap(), 32; got != want {
		t.Errorf("Cap() = %d, want %d", got, want)
	}
	if w := pool.Get(0); w != nil {
		t.Errorf("Get(0) = %v, want nil", w)
	}
	if w := pool.Get(1025); w != nil {
		t.Errorf("Get(1025) = %v, want nil", w)
	}

	empty := buffer.NewWriterPool(-1)
	if w := empty.Get(1); w != nil {
		t.Errorf("Get(1) on empty pool = %v, want nil", w)
	}
}

// TestBytesPool_Get verifies the byte pool bounds.
func TestBytesPool_Get(t *testing.T) {
	pool := buffer.NewBytesPoolWithCapacity(1024)

	if b := pool.Get(20); b == nil {
		t.Error("Get(20) = nil, want non-nil")
	} else if got, want := b.Cap(), 32; got != want {
		t.Errorf("Cap() = %d, want %d", got, want)
	}
	if b := pool.Get(0); b != nil {
		t.Errorf("Get(0) = %v, want nil", b)
	}
	if b := pool.Get(1025); b != nil {
		t.Errorf("Get(1025) = %v, want nil", b)
	}

	empty := buffer.NewBytesPool(-1)
	if b := empty.Get(1); b != nil {
		t.Errorf("Get(1) on empty pool = %v, want nil", b)
	}

	if b := buffer.MallocBytes(16); b == nil {
		t.Error("MallocBytes(16) = nil, want non-nil")
	}
	if b := buffer.MallocBytes(0); b != nil {
		t.Errorf("MallocBytes(0) = %v, want nil", b)
	}
}
