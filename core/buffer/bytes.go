package buffer

import "sync"

type Bytes struct {
	buf  []byte
	off  int
	pool *sync.Pool
}

var _ Buffer = (*Bytes)(nil)

// NewBytes 以指定buf创建字节
func NewBytes(buf []byte) *Bytes {
	return &Bytes{buf: buf, off: len(buf)}
}

// NewBytesWithCapacity 以指定容量创建字节
func NewBytesWithCapacity(cap int) *Bytes {
	return &Bytes{buf: make([]byte, cap), off: cap}
}

// Len 返回数据长度
func (b *Bytes) Len() int {
	if b == nil {
		return 0
	} else {
		return b.off
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
		return cap(b.buf) - b.off
	}
}

// Bytes 获取字节数据
func (b *Bytes) Bytes() []byte {
	if b == nil {
		return nil
	} else {
		return b.buf[:b.off]
	}
}

// Release 释放
func (b *Bytes) Release() {
	b.off = 0

	if b.pool != nil {
		b.pool.Put(b)
	}
}
