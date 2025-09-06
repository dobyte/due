package internal

import (
	"bytes"
	"sync"
)

type Buffer interface {
	Bytes() []byte

	Release()
}

type buffer struct {
	pool  *sync.Pool
	bufer *bytes.Buffer
}

func (b *buffer) Bytes() []byte {
	return b.bufer.Bytes()
}

func (b *buffer) Release() {
	b.bufer.Reset()
	b.pool.Put(b)
}

func (b *buffer) Write(p []byte) (n int, err error) {
	return b.bufer.Write(p)
}

func (b *buffer) WriteByte(c byte) error {
	return b.bufer.WriteByte(c)
}

func (b *buffer) WriteRune(r rune) (n int, err error) {
	return b.bufer.WriteRune(r)
}

func (b *buffer) WriteString(s string) (n int, err error) {
	return b.bufer.WriteString(s)
}
