package buffer

import (
	"encoding/binary"
	"math"
)

type Writer struct {
	buf []byte
	off int
}

func NewWriter(cap ...int) *Writer {
	w := &Writer{}

	if len(cap) > 0 {
		w.buf = make([]byte, cap[0])
	} else {
		w.buf = make([]byte, 0)
	}

	return w
}

// Len 返回数据长度
func (w *Writer) Len() int {
	return w.off
}

// Cap 返回容量
func (w *Writer) Cap() int {
	return cap(w.buf)
}

// Available 返回可用空间
func (w *Writer) Available() int {
	return cap(w.buf) - w.off
}

// Bytes 获取字节数据
func (w *Writer) Bytes() []byte {
	return w.buf[:w.off]
}

// Reset 复位
func (w *Writer) Reset() {
	w.off = 0
	w.buf = w.buf[:0]
}

// Grow 增长空间
func (w *Writer) Grow(n int) {
	w.growSlice(n)
}

// 写数据，实现io.Writer
func (w *Writer) Write(p []byte) (n int, err error) {
	w.grow(len(p))
	n = copy(w.buf[w.off:], p)
	return
}

// WriteBools 写入bool值
func (w *Writer) WriteBools(values ...bool) {
	w.grow(len(values))
	for _, v := range values {
		if v {
			w.buf[w.off] = 1
		} else {
			w.buf[w.off] = 0
		}
		w.off++
	}
}

// WriteInt8s 写入int8
func (w *Writer) WriteInt8s(values ...int8) {
	w.grow(len(values))
	for _, v := range values {
		w.buf[w.off] = uint8(v)
		w.off++
	}
}

// WriteUint8s 写入uint8
func (w *Writer) WriteUint8s(values ...uint8) {
	w.grow(len(values))
	for _, v := range values {
		w.buf[w.off] = v
		w.off++
	}
}

// WriteInt16s 写入int16
func (w *Writer) WriteInt16s(order binary.ByteOrder, values ...int16) {
	w.grow(b16 * len(values))
	for _, v := range values {
		order.PutUint16(w.buf[w.off:w.off+2], uint16(v))
		w.off += b16
	}
}

// WriteUint16s 写入uint16
func (w *Writer) WriteUint16s(order binary.ByteOrder, values ...uint16) {
	w.grow(b16 * len(values))
	for _, v := range values {
		order.PutUint16(w.buf[w.off:w.off+b16], v)
		w.off += b16
	}
}

// WriteInt32s 写入int32
func (w *Writer) WriteInt32s(order binary.ByteOrder, values ...int32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.off:w.off+b32], uint32(v))
		w.off += b32
	}
}

// WriteUint32s 写入uint32
func (w *Writer) WriteUint32s(order binary.ByteOrder, values ...uint32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.off:w.off+b32], v)
		w.off += b32
	}
}

// WriteInt64s 写入int64
func (w *Writer) WriteInt64s(order binary.ByteOrder, values ...int64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.off:w.off+b64], uint64(v))
		w.off += b64
	}
}

// WriteUint64s 写入uint64
func (w *Writer) WriteUint64s(order binary.ByteOrder, values ...uint64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.off:w.off+b64], v)
		w.off += b64
	}
}

// WriteFloat32s 写入float32
func (w *Writer) WriteFloat32s(order binary.ByteOrder, values ...float32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.off:w.off+b32], math.Float32bits(v))
		w.off += b32
	}
}

// WriteFloat64s 写入float64
func (w *Writer) WriteFloat64s(order binary.ByteOrder, values ...float64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.off:w.off+b64], math.Float64bits(v))
		w.off += b64
	}
}

// WriteRunes 写入rune
func (w *Writer) WriteRunes(order binary.ByteOrder, values ...rune) {
	w.WriteInt32s(order, values...)
}

// WriteString 写入字符串
func (w *Writer) WriteString(str string) {
	w.WriteBytes([]byte(str)...)
}

// WriteBytes 写入字节序
func (w *Writer) WriteBytes(values ...byte) {
	w.grow(len(values))
	copy(w.buf[w.off:], values)
	w.off += len(values)
}

// 执行扩容操作
func (w *Writer) grow(n int) {
	if w.off+n < len(w.buf) {
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
	copy(buf, w.buf[:w.off])
	w.buf = buf
}
