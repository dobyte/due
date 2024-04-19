package packet

import (
	"github.com/dobyte/due/v2/core/buffer"
	"sync"
)

type Writer struct {
	*buffer.Writer
	pool *sync.Pool
}

func NewWriter(pool *sync.Pool, cap int) *Writer {
	b := &Writer{Writer: buffer.NewWriter(cap)}
	b.pool = pool

	return b
}

func (b *Writer) Recycle() {
	b.Reset()
	b.pool.Put(b)
}
