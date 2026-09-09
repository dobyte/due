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
	pool     *sync.Pool
	released atomic.Bool
}

var _ Buffer = (*Bytes)(nil)

// NewBytes 以指定buf创建字节
func NewBytes(buf []byte) *Bytes {
	return &Bytes{buf: buf, upper: len(buf)}
}

// NewBytesWithCapacity 以指定容量创建字节
func NewBytesWithCapacity(cap int) *Bytes {
	return &Bytes{buf: make([]byte, cap), upper: cap}
}

// Len 返回数据长度
func (b *Bytes) Len() int {
	if b == nil {
		return 0
	} else {
		return b.upper - b.lower
	}
}

// Cap 返回容量
func (b *Bytes) Cap() int {
	if b == nil {
		return 0
	} else {
		return cap(b.buf)
	}
}

// Available 返回可用空间
func (b *Bytes) Available() int {
	if b == nil {
		return 0
	} else {
		return b.Cap() - b.upper
	}
}

// MoveTo 移动lower索引
func (b *Bytes) MoveTo(lower int) {
	if b != nil && lower >= 0 && lower <= b.upper {
		b.lower = lower
	}
}

// Bytes 获取字节数据
func (b *Bytes) Bytes() []byte {
	if b == nil {
		return nil
	} else {
		return b.buf[b.lower:b.upper]
	}
}

// Release 释放
func (b *Bytes) Release() {
	if !b.released.CompareAndSwap(false, true) {
		return
	}

	b.lower = 0
	b.upper = 0

	if b.pool != nil {
		b.pool.Put(b)
	}
}
