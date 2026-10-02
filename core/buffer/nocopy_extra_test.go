package buffer_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

// TestNocopyBuffer_MountKinds verifies mounting every supported block kind at both ends.
func TestNocopyBuffer_MountKinds(t *testing.T) {
	buf := buffer.NewNocopyBuffer()

	buf.Mount([]byte{})                    // empty byte slice is ignored
	buf.Mount([]byte("ab"), buffer.Head)   // head byte slice
	buf.Mount([]byte("cd"))                // tail byte slice
	buf.Mount((*buffer.Bytes)(nil))        // nil *Bytes is ignored
	buf.Mount((*buffer.Writer)(nil))       // nil *Writer is ignored
	buf.Mount((*buffer.NocopyNode)(nil))   // nil *NocopyNode is ignored
	buf.Mount((*buffer.NocopyBuffer)(nil)) // nil *NocopyBuffer is ignored
	buf.Mount(123)                         // unsupported kind is ignored
	buf.Mount(123, buffer.Head)            // unsupported kind at the head is ignored

	bytes := buffer.NewBytes([]byte("ef"))
	buf.Mount(bytes, buffer.Head)

	writer := buffer.NewWriterWithCapacity(2)
	writer.WriteBytes('g', 'h')
	buf.Mount(writer)

	if got, want := buf.Nodes(), 4; got != want {
		t.Errorf("Nodes() = %d, want %d", got, want)
	}
	if got, want := buf.Len(), 8; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
	if got, want := string(buf.Bytes()), "efabcdgh"; got != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}

	var visited int
	if ok := buf.VisitNodes(func(node *buffer.NocopyNode) bool {
		visited++
		return true
	}); !ok {
		t.Error("VisitNodes() = false, want true")
	}
	if visited != 4 {
		t.Errorf("VisitNodes() visited %d nodes, want 4", visited)
	}

	var collected string
	if ok := buf.VisitBytes(func(data []byte) bool {
		collected += string(data)
		return true
	}); !ok {
		t.Error("VisitBytes() = false, want true")
	}
	if collected != "efabcdgh" {
		t.Errorf("VisitBytes() collected %q, want %q", collected, "efabcdgh")
	}
}

// TestNocopyBuffer_NestedMount verifies mounting a buffer onto another buffer at both ends.
func TestNocopyBuffer_NestedMount(t *testing.T) {
	t.Run("nested at head with existing head", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("12"))

		outer := buffer.NewNocopyBuffer()
		outer.Mount([]byte("A"))
		outer.Mount(inner, buffer.Head)
		outer.Mount([]byte("B"))

		if got, want := outer.Nodes(), 3; got != want {
			t.Errorf("Nodes() = %d, want %d", got, want)
		}
		if got, want := outer.Len(), 4; got != want {
			t.Errorf("Len() = %d, want %d", got, want)
		}
		if got, want := string(outer.Bytes()), "12AB"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}

		var collected string
		outer.VisitBytes(func(data []byte) bool {
			collected += string(data)
			return true
		})
		if collected != "12AB" {
			t.Errorf("VisitBytes() collected %q, want %q", collected, "12AB")
		}
	})

	t.Run("nested at head on empty buffer", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("12"))

		outer := buffer.NewNocopyBuffer()
		outer.Mount(inner, buffer.Head)

		if got, want := string(outer.Bytes()), "12"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
	})

	t.Run("nested at tail with existing tail", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("12"))

		outer := buffer.NewNocopyBuffer([]byte("X"))
		outer.Mount(inner)

		if got, want := string(outer.Bytes()), "X12"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
	})

	t.Run("nested at tail on empty buffer", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("12"))

		outer := buffer.NewNocopyBuffer()
		outer.Mount(inner)

		if got, want := string(outer.Bytes()), "12"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
	})

	t.Run("node at head over nested buffer", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("12"))

		outer := buffer.NewNocopyBuffer(inner)
		outer.Mount([]byte("Z"), buffer.Head)

		if got, want := string(outer.Bytes()), "Z12"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
	})

	t.Run("node at tail over nested buffer", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("12"))

		outer := buffer.NewNocopyBuffer()
		outer.Mount(inner)
		outer.Mount([]byte("Y"))

		if got, want := string(outer.Bytes()), "12Y"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
	})
}

// TestNocopyBuffer_Slide verifies sliding plain and nested buffers.
func TestNocopyBuffer_Slide(t *testing.T) {
	t.Run("plain nodes", func(t *testing.T) {
		buf := buffer.NewNocopyBuffer([]byte("abc"), []byte("def"))

		if buf.Slide(-1) {
			t.Error("Slide(-1) = true, want false")
		}
		if buf.Slide(10) {
			t.Error("Slide(10) = true, want false")
		}
		if !buf.Slide(2) {
			t.Fatal("Slide(2) = false, want true")
		}
		if got, want := string(buf.Bytes()), "cdef"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
		if !buf.Slide(4) {
			t.Fatal("Slide(4) = false, want true")
		}
		if got := buf.Bytes(); len(got) != 0 {
			t.Errorf("Bytes() = %q, want empty", got)
		}
		if !buf.Slide(0) {
			t.Error("Slide(0) = false, want true")
		}
		if buf.Slide(1) {
			t.Error("Slide(1) on empty buffer = true, want false")
		}
	})

	t.Run("nested buffer", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("ab"))

		buf := buffer.NewNocopyBuffer([]byte("X"), inner)

		if !buf.Slide(1) {
			t.Fatal("Slide(1) = false, want true")
		}
		if got, want := string(buf.Bytes()), "ab"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}

		if !buf.Slide(1) {
			t.Fatal("Slide(1) = false, want true")
		}
		if got, want := string(buf.Bytes()), "b"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}

		if !buf.Slide(1) {
			t.Fatal("Slide(1) = false, want true")
		}
		if got := buf.Bytes(); len(got) != 0 {
			t.Errorf("Bytes() = %q, want empty", got)
		}
		if got, want := buf.Nodes(), 0; got != want {
			t.Errorf("Nodes() = %d, want %d", got, want)
		}
	})
}

// TestNocopyNode_Methods verifies the node methods for every block kind.
func TestNocopyNode_Methods(t *testing.T) {
	t.Run("bytes block", func(t *testing.T) {
		node := firstNode(t, buffer.NewNocopyBuffer(buffer.NewBytes([]byte("abcd"))))

		if got := node.Nodes(); got != 1 {
			t.Errorf("Nodes() = %d, want 1", got)
		}
		if got, want := node.Len(), 4; got != want {
			t.Errorf("Len() = %d, want %d", got, want)
		}
		if got, want := string(node.Bytes()), "abcd"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
		if !node.Slide(2) {
			t.Fatal("Slide(2) = false, want true")
		}
		if got, want := string(node.Bytes()), "cd"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}

		node.Release()
		assertReleasedNode(t, node)
	})

	t.Run("writer block", func(t *testing.T) {
		writer := buffer.NewWriterWithCapacity(4)
		writer.WriteString("xy")

		node := firstNode(t, buffer.NewNocopyBuffer(writer))

		if got, want := node.Len(), 2; got != want {
			t.Errorf("Len() = %d, want %d", got, want)
		}
		if got, want := string(node.Bytes()), "xy"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
		if !node.Slide(1) {
			t.Fatal("Slide(1) = false, want true")
		}
		if got, want := string(node.Bytes()), "y"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}

		node.Release()
		assertReleasedNode(t, node)
	})

	t.Run("byte slice block", func(t *testing.T) {
		node := firstNode(t, buffer.NewNocopyBuffer([]byte("mn")))

		if got, want := node.Len(), 2; got != want {
			t.Errorf("Len() = %d, want %d", got, want)
		}
		if got, want := string(node.Bytes()), "mn"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
		if node.Slide(-1) {
			t.Error("Slide(-1) = true, want false")
		}
		if node.Slide(3) {
			t.Error("Slide(3) = true, want false")
		}
		if !node.Slide(1) {
			t.Fatal("Slide(1) = false, want true")
		}
		if got, want := string(node.Bytes()), "n"; got != want {
			t.Errorf("Bytes() = %q, want %q", got, want)
		}
	})
}

// TestNocopyBuffer_EmptyAndNil verifies the empty and nil buffer cases.
func TestNocopyBuffer_EmptyAndNil(t *testing.T) {
	empty := buffer.NewNocopyBuffer()

	if got := empty.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
	if got := empty.Nodes(); got != 0 {
		t.Errorf("Nodes() = %d, want 0", got)
	}
	if got := empty.Bytes(); got != nil {
		t.Errorf("Bytes() = %v, want nil", got)
	}
	if !empty.VisitNodes(func(node *buffer.NocopyNode) bool { return true }) {
		t.Error("VisitNodes() = false, want true")
	}
	if !empty.VisitBytes(func(data []byte) bool { return true }) {
		t.Error("VisitBytes() = false, want true")
	}

	var nilBuf *buffer.NocopyBuffer
	if got := nilBuf.Bytes(); got != nil {
		t.Errorf("nil Bytes() = %v, want nil", got)
	}
}

// TestNocopyBuffer_VisitStop verifies that a visitor returning false stops the traversal.
func TestNocopyBuffer_VisitStop(t *testing.T) {
	t.Run("plain nodes", func(t *testing.T) {
		buf := buffer.NewNocopyBuffer([]byte("a"), []byte("b"))

		if buf.VisitNodes(func(node *buffer.NocopyNode) bool { return false }) {
			t.Error("VisitNodes() = true, want false")
		}
		if buf.VisitBytes(func(data []byte) bool { return false }) {
			t.Error("VisitBytes() = true, want false")
		}
	})

	t.Run("nested buffer", func(t *testing.T) {
		inner := buffer.NewNocopyBuffer([]byte("ab"))

		buf := buffer.NewNocopyBuffer(inner, []byte("c"))

		if buf.VisitNodes(func(node *buffer.NocopyNode) bool { return false }) {
			t.Error("VisitNodes() = true, want false")
		}
		if buf.VisitBytes(func(data []byte) bool { return false }) {
			t.Error("VisitBytes() = true, want false")
		}
	})
}

// TestNocopyBuffer_ReleaseDelayed verifies the delayed release behaviour.
func TestNocopyBuffer_ReleaseDelayed(t *testing.T) {
	buf := buffer.NewNocopyBuffer([]byte("abc"))
	buf.Delay(2)

	buf.Release()
	if got := buf.Len(); got != 3 {
		t.Errorf("Len() after first Release = %d, want 3", got)
	}

	buf.Release()
	if got := buf.Len(); got != 0 {
		t.Errorf("Len() after second Release = %d, want 0", got)
	}
	if got := buf.Bytes(); got != nil {
		t.Errorf("Bytes() after release = %v, want nil", got)
	}

	buf.Release()
	if got := buf.Nodes(); got != 0 {
		t.Errorf("Nodes() after extra Release = %d, want 0", got)
	}
}

// TestNocopyBuffer_ReleaseNested verifies releasing a buffer that mounts another buffer.
func TestNocopyBuffer_ReleaseNested(t *testing.T) {
	inner := buffer.NewNocopyBuffer([]byte("ab"))

	outer := buffer.NewNocopyBuffer(inner, []byte("cd"))
	outer.Release()

	if got := outer.Bytes(); got != nil {
		t.Errorf("Bytes() = %v, want nil", got)
	}
	if got := inner.Len(); got != 0 {
		t.Errorf("inner Len() = %d, want 0", got)
	}
}

// TestNocopyBuffer_MallocBytes verifies allocating a byte block directly onto a buffer.
func TestNocopyBuffer_MallocBytes(t *testing.T) {
	buf := buffer.NewNocopyBuffer()

	block := buf.MallocBytes(8)
	if block == nil {
		t.Fatal("MallocBytes(8) = nil, want non-nil")
	}
	if got, want := block.Cap(), 8; got != want {
		t.Errorf("Cap() = %d, want %d", got, want)
	}
	if got, want := buf.Nodes(), 1; got != want {
		t.Errorf("Nodes() = %d, want %d", got, want)
	}

	// The default byte pool accepts capacities up to 1<<32, so an out-of-range capacity does not fit
	// in a 32-bit int. Keeping the capacity in a variable defers its conversion to run time, which
	// lets this case compile on every platform; it is only exercised on 64-bit ones.
	oversized := int64(1) << 40
	if strconv.IntSize > 32 {
		if block := buf.MallocBytes(int(oversized)); block != nil {
			t.Errorf("MallocBytes(oversized) = %v, want nil", block)
		}
		if got, want := buf.Nodes(), 1; got != want {
			t.Errorf("Nodes() = %d, want %d", got, want)
		}
	}
}

// firstNode returns the first node visited on buf.
func firstNode(t *testing.T, buf *buffer.NocopyBuffer) *buffer.NocopyNode {
	t.Helper()

	var node *buffer.NocopyNode
	buf.VisitNodes(func(n *buffer.NocopyNode) bool {
		node = n
		return false
	})

	if node == nil {
		t.Fatal("buffer has no node")
	}

	return node
}

// assertReleasedNode verifies a released node reports empty results.
func assertReleasedNode(t *testing.T, node *buffer.NocopyNode) {
	t.Helper()

	if got := node.Len(); got != 0 {
		t.Errorf("Len() after Release = %d, want 0", got)
	}
	if got := node.Bytes(); got != nil {
		t.Errorf("Bytes() after Release = %v, want nil", got)
	}
	if node.Slide(1) {
		t.Error("Slide(1) after Release = true, want false")
	}

	node.Release()
}

// TestNocopyBuffer_BytesSingleNested verifies Bytes when a buffer holds exactly one nested buffer.
func TestNocopyBuffer_BytesSingleNested(t *testing.T) {
	inner := buffer.NewNocopyBuffer([]byte("ab"))

	outer := buffer.NewNocopyBuffer()
	outer.Mount(inner)

	if got, want := outer.Nodes(), 1; got != want {
		t.Errorf("Nodes() = %d, want %d", got, want)
	}
	if got, want := string(outer.Bytes()), "ab"; got != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(outer.Bytes(), []byte("ab")) {
		t.Error("Bytes() mismatch")
	}
}
