package drpc

import (
	"sync"

	"github.com/dobyte/due/v2/core/buffer"
)

type pending struct {
	calls []*calls // 分片
}

func newPending() *pending {
	p := &pending{calls: make([]*calls, 20)}

	for i := 0; i < len(p.calls); i++ {
		p.calls[i] = &calls{calls: make(map[uint64]chan buffer.Buffer)}
	}

	return p
}

// 提取
func (p *pending) extract(seq uint64) (chan buffer.Buffer, bool) {
	return p.calls[int(seq%uint64(len(p.calls)))].extract(seq)
}

// 存储
func (p *pending) store(seq uint64, call chan buffer.Buffer) {
	p.calls[int(seq%uint64(len(p.calls)))].store(seq, call)
}

// 删除
func (p *pending) delete(seq uint64) {
	p.calls[int(seq%uint64(len(p.calls)))].delete(seq)
}

type calls struct {
	mu    sync.Mutex                    // 锁
	calls map[uint64]chan buffer.Buffer // 同步通道
}

// 提取
func (p *calls) extract(seq uint64) (chan buffer.Buffer, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	call, ok := p.calls[seq]
	if ok {
		delete(p.calls, seq)
	}

	return call, ok
}

// 存储
func (p *calls) store(seq uint64, call chan buffer.Buffer) {
	p.mu.Lock()
	p.calls[seq] = call
	p.mu.Unlock()
}

// 删除
func (p *calls) delete(seq uint64) {
	p.mu.Lock()
	delete(p.calls, seq)
	p.mu.Unlock()
}
