package drpc

import (
	"sync"

	"github.com/dobyte/due/v2/core/buffer"
)

type pending struct {
	calls []*calls // 分片
}

func newPending() *pending {
	p := &pending{calls: make([]*calls, 64)}

	for i := 0; i < len(p.calls); i++ {
		p.calls[i] = &calls{calls: make(map[uint64]chan buffer.Buffer)}
	}

	return p
}

// 回复
func (p *pending) reply(seq uint64, buf buffer.Buffer) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].reply(seq, buf)
}

// 存储
func (p *pending) store(seq uint64, call chan buffer.Buffer) {
	p.calls[int(seq%uint64(len(p.calls)))].store(seq, call)
}

// 删除
func (p *pending) delete(seq uint64) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].delete(seq)
}

type calls struct {
	mu    sync.Mutex                    // 锁
	calls map[uint64]chan buffer.Buffer // 同步通道
}

// 提取
func (p *calls) reply(seq uint64, buf buffer.Buffer) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if call, ok := p.calls[seq]; ok {
		delete(p.calls, seq)

		select {
		case call <- buf:
			return true
		default:
			return false
		}
	} else {
		return false
	}
}

// 存储
func (p *calls) store(seq uint64, call chan buffer.Buffer) {
	p.mu.Lock()
	p.calls[seq] = call
	p.mu.Unlock()
}

// 删除
func (p *calls) delete(seq uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if call, ok := p.calls[seq]; ok {
		close(call)

		delete(p.calls, seq)

		return true
	} else {
		return false
	}
}
