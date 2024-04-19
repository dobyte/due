package packet

import (
	"bytes"
	"sync"
)

type Buffer struct {
	bytes.Buffer
	pool *sync.Pool
}

func NewBuffer(pool *sync.Pool, cap int) *Buffer {
	b := &Buffer{}
	b.pool = pool
	b.Grow(cap)

	return b
}

func (b *Buffer) Recycle() {
	b.Reset()
	b.pool.Put(b)
}

type IBuffer interface {
	Recycle()
	Bytes() []byte
}
