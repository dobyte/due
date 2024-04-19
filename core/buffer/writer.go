package buffer

import (
	"encoding/binary"
	"math"
)

type Writer struct {
	buf []byte
	off int
}

func NewWriter(cap int) *Writer {
	return &Writer{buf: make([]byte, cap)}
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
	w.grow(2 * len(values))

	for _, v := range values {
		order.PutUint16(w.buf[w.off:w.off+2], uint16(v))
		w.off += 2
	}
}

// WriteUint16s 写入uint16
func (w *Writer) WriteUint16s(order binary.ByteOrder, values ...uint16) {
	w.grow(2 * len(values))

	for _, v := range values {
		order.PutUint16(w.buf[w.off:w.off+2], v)
		w.off += 2
	}
}

// WriteInt32s 写入int32
func (w *Writer) WriteInt32s(order binary.ByteOrder, values ...int32) {
	w.grow(4 * len(values))

	for _, v := range values {
		order.PutUint32(w.buf[w.off:w.off+4], uint32(v))
		w.off += 4
	}
}

// WriteUint32s 写入uint32
func (w *Writer) WriteUint32s(order binary.ByteOrder, values ...uint32) {
	w.grow(4 * len(values))

	for _, v := range values {
		order.PutUint32(w.buf[w.off:w.off+4], v)
		w.off += 4
	}
}

// WriteInt64s 写入int64
func (w *Writer) WriteInt64s(order binary.ByteOrder, values ...int64) {
	w.grow(8 * len(values))

	for _, v := range values {
		order.PutUint64(w.buf[w.off:w.off+8], uint64(v))
		w.off += 8
	}
}

// WriteUint64s 写入uint64
func (w *Writer) WriteUint64s(order binary.ByteOrder, values ...uint64) {
	w.grow(8 * len(values))

	for _, v := range values {
		order.PutUint64(w.buf[w.off:w.off+8], v)
		w.off += 8
	}
}

// WriteFloat32s 写入float32
func (w *Writer) WriteFloat32s(order binary.ByteOrder, values ...float32) {
	w.grow(4 * len(values))

	for _, v := range values {
		order.PutUint32(w.buf[w.off:w.off+4], math.Float32bits(v))
		w.off += 4
	}
}

// WriteFloat64s 写入float64
func (w *Writer) WriteFloat64s(order binary.ByteOrder, values ...float64) {
	w.grow(8 * len(values))

	for _, v := range values {
		order.PutUint64(w.buf[w.off:w.off+8], math.Float64bits(v))
		w.off += 8
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
func (w *Writer) grow(size int) {
	for i := 0; i < size-(len(w.buf)-w.off); i++ {
		w.buf = append(w.buf, 0)
	}
}
