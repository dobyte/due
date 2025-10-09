package buffer

import (
	"math"
	"sync"
)

type BytesPool struct {
	pools []*sync.Pool
}

// NewBytesPool 创建字节池
func NewBytesPool(grade int) *BytesPool {
	p := &BytesPool{}
	p.pools = make([]*sync.Pool, grade+1)

	for i := range grade + 1 {
		cap := 1 << i
		p.pools[i] = &sync.Pool{New: func() any { return NewBytes(cap) }}
	}

	return p
}

// NewBytesPoolWithCapacity 创建字节池
func NewBytesPoolWithCapacity(cap int) *BytesPool {
	return NewBytesPool(int(math.Ceil(math.Log2(float64(cap)))))
}

// Get 获取
func (p *BytesPool) Get(cap int) *Bytes {
	pool := p.getPool(cap)

	if pool == nil {
		return nil
	}

	b := pool.Get().(*Bytes)
	b.off = cap

	return b
}

// Put 放回
func (p *BytesPool) Put(w *Bytes) {
	pool := p.getPool(w.Cap())

	if pool == nil {
		return
	}

	pool.Put(w)
}

// 获取对象池
func (p *BytesPool) getPool(cap int) *sync.Pool {
	if len(p.pools) == 0 {
		return nil
	}

	i := min(int(math.Ceil(math.Log2(float64(cap)))), len(p.pools)-1)

	return p.pools[i]
}
