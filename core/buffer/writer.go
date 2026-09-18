package buffer

import (
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
)

// Writer 字节写入器
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

// NewWriter 以指定buf创建写入器
func NewWriter(buf []byte, static ...bool) *Writer {
	return &Writer{buf: buf[:cap(buf)], static: len(static) > 0 && static[0]}
}

// NewWriterWithCapacity 以指定容量创建写入器
func NewWriterWithCapacity(cap ...int) *Writer {
	if len(cap) > 0 {
		return &Writer{buf: make([]byte, cap[0])}
	} else {
		return &Writer{buf: make([]byte, 0)}
	}
}

// Len 返回数据长度
func (w *Writer) Len() int {
	return w.upper - w.lower
}

// Cap 返回容量
func (w *Writer) Cap() int {
	return cap(w.buf)
}

// Available 返回可用空间
func (w *Writer) Available() int {
	return max(w.Cap()-w.upper, 0)
}

// Nodes 获取节点数
func (w *Writer) Nodes() int {
	return 1
}

// Bytes 获取字节数据
func (w *Writer) Bytes() []byte {
	return w.buf[w.lower:w.upper]
}

// VisitBytes 迭代所有字节
func (w *Writer) VisitBytes(fn func(bytes []byte) bool) bool {
	return fn(w.Bytes())
}

// Grow 增长空间
func (w *Writer) Grow(n int) {
	w.growSlice(n)
}

// Delay 设置延迟释放点
func (w *Writer) Delay(delay int) {
	w.delay.Store(int32(delay))
}

// Release 释放
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

// MoveTo 移动游标到指定位置
func (w *Writer) MoveTo(pos int) bool {
	if pos >= 0 && pos <= w.upper && pos >= w.lower {
		w.lower = pos
		return true
	}

	return false
}

// Write 写数据，实现io.Writer接口
func (w *Writer) Write(p []byte) (n int, err error) {
	w.grow(len(p))
	n = copy(w.buf[w.upper:], p)
	w.upper += n
	return
}

// WriteBools 写入bool值
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

// WriteInt8s 写入int8
func (w *Writer) WriteInt8s(values ...int8) {
	w.grow(len(values))
	for _, v := range values {
		w.buf[w.upper] = uint8(v)
		w.upper++
	}
}

// WriteUint8s 写入uint8
func (w *Writer) WriteUint8s(values ...uint8) {
	w.grow(len(values))
	for _, v := range values {
		w.buf[w.upper] = v
		w.upper++
	}
}

// WriteInt16s 写入int16
func (w *Writer) WriteInt16s(order binary.ByteOrder, values ...int16) {
	w.grow(b16 * len(values))
	for _, v := range values {
		order.PutUint16(w.buf[w.upper:w.upper+2], uint16(v))
		w.upper += b16
	}
}

// WriteUint16s 写入uint16
func (w *Writer) WriteUint16s(order binary.ByteOrder, values ...uint16) {
	w.grow(b16 * len(values))
	for _, v := range values {
		order.PutUint16(w.buf[w.upper:w.upper+b16], v)
		w.upper += b16
	}
}

// WriteInt32s 写入int32
func (w *Writer) WriteInt32s(order binary.ByteOrder, values ...int32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.upper:w.upper+b32], uint32(v))
		w.upper += b32
	}
}

// WriteUint32s 写入uint32
func (w *Writer) WriteUint32s(order binary.ByteOrder, values ...uint32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.upper:w.upper+b32], v)
		w.upper += b32
	}
}

// WriteInt64s 写入int64
func (w *Writer) WriteInt64s(order binary.ByteOrder, values ...int64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.upper:w.upper+b64], uint64(v))
		w.upper += b64
	}
}

// WriteUint64s 写入uint64
func (w *Writer) WriteUint64s(order binary.ByteOrder, values ...uint64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.upper:w.upper+b64], v)
		w.upper += b64
	}
}

// WriteFloat32s 写入float32
func (w *Writer) WriteFloat32s(order binary.ByteOrder, values ...float32) {
	w.grow(b32 * len(values))
	for _, v := range values {
		order.PutUint32(w.buf[w.upper:w.upper+b32], math.Float32bits(v))
		w.upper += b32
	}
}

// WriteFloat64s 写入float64
func (w *Writer) WriteFloat64s(order binary.ByteOrder, values ...float64) {
	w.grow(b64 * len(values))
	for _, v := range values {
		order.PutUint64(w.buf[w.upper:w.upper+b64], math.Float64bits(v))
		w.upper += b64
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
	copy(w.buf[w.upper:], values)
	w.upper += len(values)
}

// 执行扩容操作
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
