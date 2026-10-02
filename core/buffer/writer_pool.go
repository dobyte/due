package buffer

import (
	"math/bits"
	"sync"
)

var defaultWriterPool = NewWriterPool(32)

// MallocWriter allocates a Writer with the given capacity from the default writer pool.
func MallocWriter(cap int) *Writer {
	return defaultWriterPool.Get(cap)
}

// WriterPool is a byte writer pool.
type WriterPool struct {
	pools []*sync.Pool
}

// NewWriterPool creates a writer pool with the given number of grades.
func NewWriterPool(grade int) *WriterPool {
	p := &WriterPool{}
	p.pools = make([]*sync.Pool, grade+1)

	for i := range grade + 1 {
		cap := 1 << i
		pool := &sync.Pool{}
		pool.New = func() any { return &Writer{buf: make([]byte, cap), pool: pool} }
		p.pools[i] = pool
	}

	return p
}

// NewWriterPoolWithCapacity creates a writer pool whose maximum capacity fits cap.
func NewWriterPoolWithCapacity(cap int) *WriterPool {
	return NewWriterPool(bits.Len(uint(max(1, cap) - 1)))
}

// Get returns a Writer with the given capacity, or nil when no matching pool exists.
func (p *WriterPool) Get(cap int) *Writer {
	pool := p.getPool(cap)

	if pool == nil {
		return nil
	}

	w := pool.Get().(*Writer)
	w.lower = 0
	w.upper = 0
	w.delay.Store(0)
	w.released.Store(false)

	return w
}

// getPool returns the pool matching cap, or nil when cap is out of range.
func (p *WriterPool) getPool(cap int) *sync.Pool {
	if cap <= 0 {
		return nil
	}

	if len(p.pools) == 0 {
		return nil
	}

	if cap > 1<<(len(p.pools)-1) {
		return nil
	}

	return p.pools[bits.Len(uint(cap-1))]
}
