package buffer

import (
	"math/bits"
	"sync"
)

var defaultBytesPool = NewBytesPool(32)

// MallocBytes allocates a Bytes with the given capacity from the default byte pool.
func MallocBytes(cap int) *Bytes {
	return defaultBytesPool.Get(cap)
}

// BytesPool is a byte buffer pool.
type BytesPool struct {
	pools []*sync.Pool
}

// NewBytesPool creates a byte pool with the given number of grades.
func NewBytesPool(grade int) *BytesPool {
	p := &BytesPool{}
	p.pools = make([]*sync.Pool, grade+1)

	for i := range grade + 1 {
		cap := 1 << i
		pool := &sync.Pool{}
		pool.New = func() any { return &Bytes{buf: make([]byte, cap), upper: cap, pool: pool} }
		p.pools[i] = pool
	}

	return p
}

// NewBytesPoolWithCapacity creates a byte pool whose maximum capacity fits cap.
func NewBytesPoolWithCapacity(cap int) *BytesPool {
	return NewBytesPool(bits.Len(uint(max(1, cap) - 1)))
}

// Get returns a Bytes with the given capacity, or nil when no matching pool exists.
func (p *BytesPool) Get(cap int) *Bytes {
	pool := p.getPool(cap)

	if pool == nil {
		return nil
	}

	b := pool.Get().(*Bytes)
	b.lower = 0
	b.upper = cap
	b.delay.Store(0)
	b.released.Store(false)

	return b
}

// getPool returns the pool matching cap, or nil when cap is out of range.
func (p *BytesPool) getPool(cap int) *sync.Pool {
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
