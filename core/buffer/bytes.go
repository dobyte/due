package buffer

import (
	"sync"
	"sync/atomic"
)

// Bytes 字节缓冲
type Bytes struct {
	buf      []byte
	lower    int
	upper    int
	static   bool
	pool     *sync.Pool
	released atomic.Bool
}

var _ Buffer = (*Bytes)(nil)

// NewBytes 以指定buf创建字节
func NewBytes(buf []byte, static ...bool) *Bytes {
	return &Bytes{buf: buf, upper: len(buf), static: len(static) > 0 && static[0]}
}

// NewBytesWithCapacity 以指定容量创建字节
func NewBytesWithCapacity(cap int) *Bytes {
	return &Bytes{buf: make([]byte, cap), upper: cap}
}

// Len 返回数据长度
func (b *Bytes) Len() int {
	return b.upper - b.lower
}

// Cap 返回容量
func (b *Bytes) Cap() int {
	return cap(b.buf)
}

// Available 返回可用空间
func (b *Bytes) Available() int {
	return b.Cap() - b.upper
}

// MoveTo 移动lower索引
func (b *Bytes) MoveTo(lower int) {
	if lower >= 0 && lower <= b.upper {
		b.lower = lower
	}
}

// Bytes 获取字节数据
func (b *Bytes) Bytes() []byte {
	return b.buf[b.lower:b.upper]
}

// Release 释放
func (b *Bytes) Release() {
	if b.static {
		return
	}

	if !b.released.CompareAndSwap(false, true) {
		return
	}

	b.lower = 0
	b.upper = 0

	if b.pool != nil {
		b.pool.Put(b)
	}
}
