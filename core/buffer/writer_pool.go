package buffer

import (
	"sync"
)

type WriterPool struct {
	pools      []*sync.Pool
	capacities []int
}

func NewWriterPool(capacities []int) *WriterPool {
	p := &WriterPool{}
	p.pools = make([]*sync.Pool, len(capacities))
	p.capacities = capacities
	for i := range capacities {
		c := capacities[i]
		p.pools[i] = &sync.Pool{New: func() any { return NewWriter(c) }}
	}

	return p
}

// Get 获取
func (p *WriterPool) Get(cap int) *Writer {
	pool := p.getPool(cap)
	return pool.Get().(*Writer)
}

// Put 放回
func (p *WriterPool) Put(w *Writer) {
	pool := p.getPool(w.Cap())
	pool.Put(w)
}

// 获取对象池
func (p *WriterPool) getPool(cap int) *sync.Pool {
	for i, c := range p.capacities {
		if cap <= c {
			return p.pools[i]
		}
	}
	return p.pools[len(p.pools)-1]
}
