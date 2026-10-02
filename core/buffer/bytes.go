package buffer

import (
	"sync"
	"sync/atomic"
)

// Bytes is a byte buffer.
type Bytes struct {
	buf      []byte
	lower    int
	upper    int
	static   bool
	delay    atomic.Int32
	pool     *sync.Pool
	released atomic.Bool
}

var _ Buffer = (*Bytes)(nil)

// NewBytes creates a Bytes over buf. When static is true, the buffer is not returned to the pool
// on release.
func NewBytes(buf []byte, static ...bool) *Bytes {
	return &Bytes{buf: buf, upper: len(buf), static: len(static) > 0 && static[0]}
}

// NewBytesWithCapacity creates a Bytes with the given capacity.
func NewBytesWithCapacity(cap int) *Bytes {
	return &Bytes{buf: make([]byte, cap), upper: cap}
}

// Len returns the data length.
func (b *Bytes) Len() int {
	return b.upper - b.lower
}

// Cap returns the capacity.
func (b *Bytes) Cap() int {
	return cap(b.buf)
}

// Available returns the available space.
func (b *Bytes) Available() int {
	return b.Cap() - b.upper
}

// Slide slides the lower index.
func (b *Bytes) Slide(delta int) bool {
	if delta >= 0 && delta+b.lower <= b.upper {
		b.lower += delta
		return true
	}

	return false
}

// Nodes returns the number of nodes.
func (b *Bytes) Nodes() int {
	return 1
}

// Bytes returns the byte data.
func (b *Bytes) Bytes() []byte {
	return b.buf[b.lower:b.upper]
}

// VisitBytes iterates over all bytes.
func (b *Bytes) VisitBytes(fn func(bytes []byte) bool) bool {
	return fn(b.Bytes())
}

// Delay sets the delayed release point.
func (b *Bytes) Delay(delay int) {
	b.delay.Store(int32(delay))
}

// Release releases the buffer.
func (b *Bytes) Release() {
	if b.static {
		return
	}

	if b.delay.Add(-1) > 0 {
		return
	}

	if !b.released.CompareAndSwap(false, true) {
		return
	}

	b.lower = 0
	b.upper = 0
	b.delay.Store(0)

	if b.pool != nil {
		b.pool.Put(b)
	}
}
