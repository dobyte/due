package buffer

import (
	"math"
	"sync"
)

var defaultWriterPool = NewWriterPool(32)

// MallocWriter 分配一块内存给Writer
func MallocWriter(cap int) *Writer {
	return defaultWriterPool.Get(cap)
}

// WriterPool 写入器池
type WriterPool struct {
	pools []*sync.Pool
}

// NewWriterPool 分级创建写入器池
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

// NewWriterPoolWithCapacity 以指定容量创建写入器池
func NewWriterPoolWithCapacity(cap int) *WriterPool {
	return NewWriterPool(int(math.Ceil(math.Log2(float64(max(1, cap))))))
}

// Get 获取
func (p *WriterPool) Get(cap int) *Writer {
	pool := p.getPool(cap)

	if pool == nil {
		return nil
	}

	w := pool.Get().(*Writer)
	w.released.Store(false)

	return w
}

// getPool 获取对象池
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

	i := min(int(math.Ceil(math.Log2(float64(cap)))), len(p.pools)-1)

	return p.pools[i]
}
