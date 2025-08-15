package log

import (
	"bytes"
	"sync"
)

type Buffer struct {
	pool  *sync.Pool
	bufer *bytes.Buffer
}

func (b *Buffer) Bytes() []byte {
	return b.bufer.Bytes()
}

func (b *Buffer) Release() {
	b.bufer.Reset()
	b.pool.Put(b)
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	return b.bufer.Write(p)
}

func (b *Buffer) WriteByte(c byte) error {
	return b.bufer.WriteByte(c)
}

func (b *Buffer) WriteString(s string) (n int, err error) {
	return b.bufer.WriteString(s)
}
