package buffer

import (
	"sync/atomic"
)

// NocopyBuffer is a zero-copy buffer.
type NocopyBuffer struct {
	len      atomic.Int64 // Byte count
	num      int          // Node count
	head     any          // Head node
	tail     any          // Tail node
	next     any          // Next node
	delay    atomic.Int32 // Delayed release point
	released atomic.Bool  // Released
}

var _ Buffer = &NocopyBuffer{}

// NewNocopyBuffer creates a zero-copy buffer mounting the given blocks.
func NewNocopyBuffer(blocks ...any) *NocopyBuffer {
	buf := &NocopyBuffer{}
	buf.len.Store(-1)

	for _, block := range blocks {
		buf.Mount(block)
	}

	return buf
}

// Len returns the byte length.
func (b *NocopyBuffer) Len() int {
	if cached := b.len.Load(); cached >= 0 {
		return int(cached)
	}

	size := 0

	for node := b.head; node != nil; {
		switch n := node.(type) {
		case *NocopyNode:
			size += n.Len()
			node = n.next
		case *NocopyBuffer:
			size += n.Len()
			node = n.next
		default:
			node = nil
		}
	}

	b.len.Store(int64(size))

	return size
}

// Mount mounts block onto the buffer, at the head or tail according to whence.
func (b *NocopyBuffer) Mount(block any, whence ...Whence) {
	switch v := block.(type) {
	case []byte:
		if len(v) == 0 {
			return
		}

		if len(whence) > 0 && whence[0] == Head {
			b.addToHead(&NocopyNode{block: v})
		} else {
			b.addToTail(&NocopyNode{block: v})
		}
	case *Bytes:
		if v == nil {
			return
		}

		if len(whence) > 0 && whence[0] == Head {
			b.addToHead(&NocopyNode{block: v})
		} else {
			b.addToTail(&NocopyNode{block: v})
		}
	case *Writer:
		if v == nil {
			return
		}

		if len(whence) > 0 && whence[0] == Head {
			b.addToHead(&NocopyNode{block: v})
		} else {
			b.addToTail(&NocopyNode{block: v})
		}
	default:
		if len(whence) > 0 && whence[0] == Head {
			b.addToHead(v)
		} else {
			b.addToTail(v)
		}
	}
}

// MallocBytes allocates a Bytes with the given capacity and mounts it.
func (b *NocopyBuffer) MallocBytes(cap int, whence ...Whence) *Bytes {
	block := MallocBytes(cap)

	if block != nil {
		b.Mount(block, whence...)
	}

	return block
}

// MallocWriter allocates a Writer with the given capacity and mounts it.
func (b *NocopyBuffer) MallocWriter(cap int, whence ...Whence) *Writer {
	block := MallocWriter(cap)

	if block != nil {
		b.Mount(block, whence...)
	}

	return block
}

// Nodes returns the number of nodes.
func (b *NocopyBuffer) Nodes() int {
	return b.num
}

// VisitNodes iterates over all nodes.
func (b *NocopyBuffer) VisitNodes(fn func(node *NocopyNode) bool) bool {
	for node := b.head; node != nil; {
		switch n := node.(type) {
		case *NocopyNode:
			next := n.next

			if !fn(n) {
				return false
			}

			node = next
		case *NocopyBuffer:
			next := n.next

			if !n.VisitNodes(fn) {
				return false
			}

			node = next
		default:
			return false
		}
	}

	return true
}

// Bytes returns all bytes.
func (b *NocopyBuffer) Bytes() []byte {
	if b == nil {
		return nil
	}

	switch b.num {
	case 0:
		return nil
	case 1:
		switch h := b.head.(type) {
		case *NocopyNode:
			return h.Bytes()
		case *NocopyBuffer:
			return h.Bytes()
		default:
			return nil
		}
	default:
		bytes := make([]byte, 0, b.Len())

		for node := b.head; node != nil; {
			switch n := node.(type) {
			case *NocopyNode:
				bytes = append(bytes, n.Bytes()...)
				node = n.next
			case *NocopyBuffer:
				bytes = append(bytes, n.Bytes()...)
				node = n.next
			default:
				return bytes
			}
		}

		return bytes
	}
}

// VisitBytes iterates over all bytes.
func (b *NocopyBuffer) VisitBytes(fn func(bytes []byte) bool) bool {
	for node := b.head; node != nil; {
		switch n := node.(type) {
		case *NocopyNode:
			next := n.next

			if !fn(n.Bytes()) {
				return false
			}

			node = next
		case *NocopyBuffer:
			next := n.next

			if !n.VisitBytes(fn) {
				return false
			}

			node = next
		default:
			return false
		}
	}

	return true
}

// Delay sets the delayed release point.
func (b *NocopyBuffer) Delay(delay int) {
	b.delay.Store(int32(delay))
}

// Release releases the buffer.
func (b *NocopyBuffer) Release() {
	if b.delay.Add(-1) > 0 {
		return
	}

	if !b.released.CompareAndSwap(false, true) {
		return
	}

	for node := b.head; node != nil; {
		switch n := node.(type) {
		case *NocopyNode:
			next := n.next
			n.Release()
			node = next
		case *NocopyBuffer:
			next := n.next
			n.Release()
			node = next
		default:
			goto OVER
		}
	}

OVER:
	b.len.Store(-1)
	b.num = 0
	b.head = nil
	b.tail = nil
	b.next = nil
}

// Slide slides the lower index.
func (b *NocopyBuffer) Slide(delta int) bool {
	if delta < 0 {
		return false
	}

	if delta > b.Len() {
		return false
	}

	remaining := delta

	for remaining > 0 {
		switch n := b.head.(type) {
		case *NocopyNode:
			size := n.Len()
			if size <= remaining {
				b.removeHead()
				n.Release()
				remaining -= size
			} else {
				n.Slide(remaining)
				remaining = 0
			}
		case *NocopyBuffer:
			size := n.Len()
			if size <= remaining {
				b.removeHead()
				n.Release()
				remaining -= size
			} else {
				n.Slide(remaining)
				remaining = 0
			}
		default:
			return false
		}
	}

	b.len.Store(-1)

	return true
}

// addToHead adds node to the head.
func (b *NocopyBuffer) addToHead(node any) {
	switch n := node.(type) {
	case *NocopyNode:
		if n == nil {
			return
		}

		if b.head == nil {
			b.head = n
			b.tail = n
		} else {
			n.next = b.head
			b.head = n
		}

		b.len.Store(-1)
		b.num++
	case *NocopyBuffer:
		if n == nil {
			return
		}

		if b.head == nil {
			b.head = n
			b.tail = n
		} else {
			n.next = b.head
			b.head = n
		}

		b.len.Store(-1)
		b.num += n.num
	default:
		// ignore
	}
}

// addToTail adds node to the tail.
func (b *NocopyBuffer) addToTail(node any) {
	switch n := node.(type) {
	case *NocopyNode:
		if n == nil {
			return
		}

		if b.tail == nil {
			b.head = n
			b.tail = n
		} else {
			switch t := b.tail.(type) {
			case *NocopyNode:
				t.next = n
				b.tail = n
			case *NocopyBuffer:
				t.next = n
				b.tail = n
			}
		}

		b.len.Store(-1)
		b.num++
	case *NocopyBuffer:
		if n == nil {
			return
		}

		if b.tail == nil {
			b.head = n
			b.tail = n
		} else {
			switch t := b.tail.(type) {
			case *NocopyNode:
				t.next = n
				b.tail = n
			case *NocopyBuffer:
				t.next = n
				b.tail = n
			}
		}

		b.len.Store(-1)
		b.num += n.num
	default:
		// ignore
	}
}

// removeHead removes the head node.
func (b *NocopyBuffer) removeHead() {
	switch n := b.head.(type) {
	case *NocopyNode:
		b.head = n.next
		b.num--
	case *NocopyBuffer:
		b.head = n.next
		b.num -= n.num
	}

	if b.head == nil {
		b.tail = nil
	}

	b.len.Store(-1)
}
