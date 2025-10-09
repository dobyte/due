package buffer

import (
	"math"
	"sync"
)

type WriterPool struct {
	pools []*sync.Pool
}

func NewWriterPool(grade int) *WriterPool {
	p := &WriterPool{}
	p.pools = make([]*sync.Pool, grade+1)

	for i := range grade + 1 {
		cap := 1 << i
		p.pools[i] = &sync.Pool{New: func() any { return NewWriter(cap) }}
	}

	return p
}

// Get 获取
func (p *WriterPool) Get(cap int) *Writer {
	pool := p.getPool(cap)

	if pool == nil {
		return nil
	}

	w := pool.Get().(*Writer)
	w.pool = p

	return w
}

// Put 放回
func (p *WriterPool) Put(w *Writer) {
	pool := p.getPool(w.Cap())

	if pool == nil {
		return
	}

	pool.Put(w)
}

// 获取对象池
func (p *WriterPool) getPool(cap int) *sync.Pool {
	if len(p.pools) == 0 {
		return nil
	}

	i := min(int(math.Ceil(math.Log2(float64(cap)))), len(p.pools)-1)

	return p.pools[i]
}
