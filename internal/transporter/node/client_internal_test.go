package node

import (
	"math"
	"testing"
)

// TestClientGenSequenceWrap verifies that the sequence generator skips the zero value.
func TestClientGenSequenceWrap(t *testing.T) {
	c := &Client{}
	c.seq.Store(math.MaxUint64)

	if seq := c.doGenSequence(); seq != 1 {
		t.Fatalf("expect sequence 1 after wrap, got %d", seq)
	}
}
