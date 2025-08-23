package internal

import (
	"bytes"
	"sync"

	"github.com/dobyte/due/v2/utils/xconv"
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

func (b *buffer) WriteString(s string) (n int, err error) {
	return b.bufer.Write(xconv.StringToBytes(s))
}
