package buffer

import (
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
)

// Writer is a byte writer.
type Writer struct {
	buf      []byte
	lower    int
	upper    int
	static   bool
	delay    atomic.Int32
	pool     *sync.Pool
	released atomic.Bool
}

var _ Buffer = (*Writer)(nil)

// NewWriter creates a Writer over buf. When static is true, the writer is not returned to the pool
// on release.
func NewWriter(buf []byte, static ...bool) *Writer {
	return &Writer{buf: buf[:cap(buf)], static: len(static) > 0 && static[0]}
}

// NewWriterWithCapacity creates a Writer with the given capacity.
func NewWriterWithCapacity(cap ...int) *Writer {
	if len(cap) > 0 {
		return &Writer{buf: make([]byte, cap[0])}
	} else {
		return &Writer{buf: make([]byte, 0)}
	}
}

// Len returns the data length.
func (w *Writer) Len() int {
	return w.upper - w.lower
}

// Cap returns the capacity.
func (w *Writer) Cap() int {
	return cap(w.buf)
}

// Available returns the available space.
func (w *Writer) Available() int {
	return max(w.Cap()-w.upper, 0)
}

// Nodes returns the number of nodes.
func (w *Writer) Nodes() int {
	return 1
}

// Bytes returns the byte data.
func (w *Writer) Bytes() []byte {
	return w.buf[w.lower:w.upper]
}

// VisitBytes iterates over all bytes.
func (w *Writer) VisitBytes(fn func(bytes []byte) bool) bool {
	return fn(w.Bytes())
}

// Grow grows the available space by n bytes.
func (w *Writer) Grow(n int) {
	w.growSlice(n)
}

// Delay sets the delayed release point.
func (w *Writer) Delay(delay int) {
	w.delay.Store(int32(delay))
}

// Release releases the writer.
func (w *Writer) Release() {
	if w.static {
		return
	}

	if w.delay.Add(-1) > 0 {
		return
	}

	if !w.released.CompareAndSwap(false, true) {
		return
	}

	w.lower = 0
	w.upper = 0
	w.delay.Store(0)

	if w.pool != nil {
		w.pool.Put(w)
	}
}

// Slide slides the lower index.
func (w *Writer) Slide(delta int) bool {
	if delta >= 0 && delta+w.lower <= w.upper {
		w.lower += delta
		return true
	}

	return false
}

// Write appends p and implements [io.Writer].
func (w *Writer) Write(p []byte) (n int, err error) {
	w.grow(len(p))
	n = copy(w.buf[w.upper:], p)
	w.upper += n
	return
}

// WriteBools writes the given bool values.
func (w *Writer) WriteBools(values ...bool) {
	w.grow(len(values))
	for _, v := range values {
		if v {
			w.buf[w.upper] = 1
		} else {
			w.buf[w.upper] = 0
		}
		w.upper++
	}
}

// WriteInt8s writes the given int8 values.
func (w *Writer) WriteInt8s(values ...int8) {
	w.grow(len(values))
	for _, v := range values {
		w.buf[w.upper] = uint8(v)
		w.upper++
	}
}

// WriteUint8s writes the given uint8 values.
func (w *Writer) WriteUint8s(values ...uint8) {
	w.grow(len(values))
	for _, v := range values {
		w.buf[w.upper] = v
		w.upper++
	}
}

// WriteInt16s writes the given int16 values using order.
func (w *Writer) WriteInt16s(order binary.ByteOrder, values ...int16) {
	w.grow(b16 * len(values))
	for _, v := range values {
		order.PutUint16(w.buf[w.upper:w.upper+2], uint16(v))
		w.upper += b16
	}
}

// WriteUint16s writes the given uint16 values using order.
func (w *Writer) WriteUint16s(order binary.ByteOrder, values ...uint16) {
	w.grow(b16 * len(values))
	for _, v := range values {
		order.PutUint16(w.buf[w.upper:w.upper+b16], v)
		w.upper += b16
	}
}

// WriteInt32s writes the given int32 values using order.
func (w *Writer) WriteInt32s(order binary.ByteOrder, values ...int32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.upper:w.upper+b32], uint32(v))
		w.upper += b32
	}
}

// WriteUint32s writes the given uint32 values using order.
func (w *Writer) WriteUint32s(order binary.ByteOrder, values ...uint32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.upper:w.upper+b32], v)
		w.upper += b32
	}
}

// WriteInt64s writes the given int64 values using order.
func (w *Writer) WriteInt64s(order binary.ByteOrder, values ...int64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.upper:w.upper+b64], uint64(v))
		w.upper += b64
	}
}

// WriteUint64s writes the given uint64 values using order.
func (w *Writer) WriteUint64s(order binary.ByteOrder, values ...uint64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.upper:w.upper+b64], v)
		w.upper += b64
	}
}

// WriteFloat32s writes the given float32 values using order.
func (w *Writer) WriteFloat32s(order binary.ByteOrder, values ...float32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.upper:w.upper+b32], math.Float32bits(v))
		w.upper += b32
	}
}

// WriteFloat64s writes the given float64 values using order.
func (w *Writer) WriteFloat64s(order binary.ByteOrder, values ...float64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.upper:w.upper+b64], math.Float64bits(v))
		w.upper += b64
	}
}

// WriteRunes writes the given rune values using order.
func (w *Writer) WriteRunes(order binary.ByteOrder, values ...rune) {
	w.WriteInt32s(order, values...)
}

// WriteString writes str.
func (w *Writer) WriteString(str string) {
	w.WriteBytes([]byte(str)...)
}

// WriteBytes writes the given bytes.
func (w *Writer) WriteBytes(values ...byte) {
	w.grow(len(values))
	copy(w.buf[w.upper:], values)
	w.upper += len(values)
}

// grow ensures that n more bytes fit in the buffer.
func (w *Writer) grow(n int) {
	if w.upper+n <= cap(w.buf) {
		return
	}

	w.growSlice(n)
}

func (w *Writer) growSlice(n int) {
	c := len(w.buf) + n

	if c < 2*cap(w.buf) {
		c = 2 * cap(w.buf)
	}

	buf := make([]byte, c)
	copy(buf, w.buf[:w.upper])
	w.buf = buf
}
