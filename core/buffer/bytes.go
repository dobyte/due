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
	headroom int
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

// Prepend exposes size bytes of reserved headroom and returns that prefix. It returns nil when the
// buffer is shared, its view has moved, or it does not have enough headroom.
func (b *Bytes) Prepend(size int) []byte {
	if size < 0 || size > b.headroom || b.lower != b.headroom || b.delay.Load() > 1 {
		return nil
	}

	b.lower -= size
	b.headroom -= size

	return b.buf[b.lower : b.lower+size]
}

// Slide slides the lower index.
func (b *Bytes) Slide(delta int) bool {
	if delta >= 0 && delta+b.lower <= b.upper {
		b.lower += delta
		return true
	}

	return false
}

// Truncate keeps the first size unread bytes. It returns false when size is out of range.
func (b *Bytes) Truncate(size int) bool {
	if size < 0 || size > b.Len() {
		return false
	}
	b.upper = b.lower + size
	return true
}

// Resize sets the readable length to size and reports false when size is out of range. The reserved
// headroom is kept intact so that a message header can still be prepended.
func (b *Bytes) Resize(size int) bool {
	if size < 0 || b.lower+size > len(b.buf) {
		return false
	}
	b.upper = b.lower + size
	return true
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
	b.headroom = 0
	b.delay.Store(0)

	if b.pool != nil {
		b.pool.Put(b)
	}
}
