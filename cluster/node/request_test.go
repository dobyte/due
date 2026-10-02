package node

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

// newTestRequestNode creates a test node that only initializes the request object pool.
func newTestRequestNode() *Node {
	n := &Node{}
	n.reqPool = &sync.Pool{New: func() any { return &request{node: n} }}
	return n
}

func TestRequestCloneBytes(t *testing.T) {
	n := newTestRequestNode()

	r := &request{node: n, message: []byte("hello")}
	c := r.Clone().(*request)

	msg, ok := c.message.([]byte)
	if !ok {
		t.Fatalf("expect []byte message, got %T", c.message)
	}
	if string(msg) != "hello" {
		t.Fatalf("expect cloned message %q, got %q", "hello", string(msg))
	}

	// The clone must own an independent slice so that modifications do not affect each other.
	msg[0] = 'x'
	if r.message.([]byte)[0] == 'x' {
		t.Fatal("clone should own an independent slice")
	}
}

func TestRequestCloneBuffer(t *testing.T) {
	n := newTestRequestNode()

	buf := buffer.NewBytes([]byte("hello"))
	defer buf.Release()

	r := &request{node: n, message: buf}
	c := r.Clone().(*request)

	msg, ok := c.message.([]byte)
	if !ok {
		t.Fatalf("expect []byte message, got %T", c.message)
	}
	if string(msg) != "hello" {
		t.Fatalf("expect cloned message %q, got %q", "hello", string(msg))
	}
}

// countJSONMarshaler is a test message type that counts the number of marshals.
type countJSONMarshaler struct {
	n *atomic.Int32
}

func (c countJSONMarshaler) MarshalJSON() ([]byte, error) {
	c.n.Add(1)
	return []byte(`{"a":1}`), nil
}

func TestRequestCloneMarshalOnce(t *testing.T) {
	n := newTestRequestNode()

	var count atomic.Int32
	r := &request{node: n, message: countJSONMarshaler{n: &count}}

	c1 := r.Clone().(*request)
	c2 := r.Clone().(*request)

	// The underlying sonic library may call MarshalJSON multiple times for a single Marshal; here
	// we assert that the second clone hits the cache and that the marshal count stops growing.
	if got := count.Load(); got == 0 {
		t.Fatal("expect marshal at least once")
	}
	first := count.Load()
	_ = r.Clone()
	if got := count.Load(); got != first {
		t.Fatalf("expect cache reuse, marshal count changed from %d to %d", first, got)
	}

	m1, ok := c1.message.([]byte)
	if !ok || string(m1) != `{"a":1}` {
		t.Fatalf("unexpected cloned message: %v", c1.message)
	}
	m2, ok := c2.message.([]byte)
	if !ok || string(m2) != `{"a":1}` {
		t.Fatalf("unexpected cloned message: %v", c2.message)
	}

	// The clones own independent slices so that modifications do not affect each other.
	m1[0] = 'x'
	if m2[0] == 'x' {
		t.Fatal("clones should own independent slices")
	}
}

func TestRequestReleaseResetCache(t *testing.T) {
	n := newTestRequestNode()

	var count atomic.Int32
	r := &request{node: n, message: countJSONMarshaler{n: &count}}
	_ = r.Clone()

	if r.cache == nil {
		t.Fatal("cache should be filled after clone")
	}

	r.release()

	if r.cache != nil {
		t.Fatal("cache should be reset after release")
	}

	recycled := n.reqPool.Get().(*request)
	if recycled.cache != nil {
		t.Fatal("recycled request should not carry stale cache")
	}
}
